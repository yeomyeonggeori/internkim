package filestream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

const (
	quietWait      = 300 * time.Millisecond
	longestWait    = 2 * time.Second
	fullEntryEvery = 24 * time.Hour
)

type Follower struct {
	Roots                []hostbackup.FileRoot
	Journal              Journal
	Keyring              box.Keyring
	ScratchDirectoryPath string
	Barrier              *Barrier
}

type following struct {
	Follower
	uploader   *offsite.Snapshot
	next       int64
	referenced map[string]bool
	lastFull   time.Time
}

func (follower Follower) Run(ctx context.Context) error {
	watcher, errorValue := NewWatcher(follower.Roots)
	if errorValue != nil {
		return errorValue
	}
	defer watcher.Close()
	uploader, errorValue := offsite.StartSnapshot(ctx, follower.Journal.chunkDestination(), follower.Keyring)
	if errorValue != nil {
		return errorValue
	}
	sequences, errorValue := follower.Journal.Sequences(ctx)
	if errorValue != nil {
		return errorValue
	}
	state := &following{Follower: follower, uploader: uploader, next: 1}
	if len(sequences) > 0 {
		state.next = slices.Max(sequences) + 1
	}
	if errorValue := state.keepFull(ctx); errorValue != nil {
		return errorValue
	}
	changes := make(chan []Change)
	watching := make(chan error, 1)
	go func() {
		for {
			found, errorValue := watcher.Next(ctx)
			if errorValue != nil {
				watching <- errorValue
				return
			}
			select {
			case changes <- found:
			case <-ctx.Done():
				watching <- ctx.Err()
				return
			}
		}
	}()
	return state.follow(ctx, changes, watching)
}

type gathering struct {
	changes      map[string]Change
	markers      []string
	isOverflowed bool
	firstChange  time.Time
}

func newGathering() gathering {
	return gathering{changes: map[string]Change{}}
}

func (gathered gathering) isEmpty() bool {
	return len(gathered.changes) == 0 && len(gathered.markers) == 0 && !gathered.isOverflowed
}

func (state *following) follow(ctx context.Context, changes <-chan []Change, watching <-chan error) error {
	gathered := newGathering()
	for {
		wait := time.Hour
		if !gathered.isEmpty() {
			wait = min(quietWait, time.Until(gathered.firstChange.Add(longestWait)))
			if len(gathered.markers) > 0 {
				wait = 0
			}
		}
		select {
		case errorValue := <-watching:
			return errorValue
		case found := <-changes:
			if gathered.isEmpty() {
				gathered.firstChange = time.Now()
			}
			state.gather(&gathered, found)
			continue
		case <-time.After(max(wait, 0)):
		}
		if errorValue := state.keepWhatChanged(ctx, gathered); errorValue != nil {
			return errorValue
		}
		gathered = newGathering()
	}
}

func (state *following) gather(gathered *gathering, found []Change) {
	for _, change := range found {
		if change.IsOverflow {
			gathered.isOverflowed = true
			continue
		}
		if marker, isBarrier := state.Barrier.markerOf(change); isBarrier {
			if marker != "" {
				gathered.markers = append(gathered.markers, marker)
			}
			continue
		}
		gathered.changes[change.archivedName()] = change
	}
}

func (state *following) keepWhatChanged(ctx context.Context, gathered gathering) error {
	switch {
	case gathered.isOverflowed:
		log.Printf("the kernel dropped file changes it could not queue, so the whole tree is kept again")
		if errorValue := state.keepFull(ctx); errorValue != nil {
			return errorValue
		}
	case time.Since(state.lastFull) >= fullEntryEvery:
		if errorValue := state.keepFull(ctx); errorValue != nil {
			return errorValue
		}
	case len(gathered.changes) > 0:
		if errorValue := state.keepChanges(ctx, gathered.changes); errorValue != nil {
			return errorValue
		}
	}
	state.Barrier.release(gathered.markers)
	return nil
}

func (state *following) keepChanges(ctx context.Context, gathered map[string]Change) error {
	names := make([]string, 0, len(gathered))
	for name := range gathered {
		names = append(names, name)
	}
	slices.Sort(names)
	present := []hostbackup.ChangedEntry{}
	removed := []string{}
	for _, name := range names {
		change := gathered[name]
		_, errorValue := os.Lstat(filepath.Join(change.Root.Path, filepath.FromSlash(change.Relative)))
		if errors.Is(errorValue, fs.ErrNotExist) {
			removed = append(removed, name)
			continue
		}
		if errorValue != nil {
			return errorValue
		}
		present = append(present, hostbackup.ChangedEntry{Root: change.Root, Relative: change.Relative})
	}
	return state.keep(ctx, false, removed, func(output io.Writer, snapshots hostbackup.Snapshotter) error {
		_, errorValue := hostbackup.WriteEntries(output, present, snapshots)
		return errorValue
	})
}

func (state *following) keepFull(ctx context.Context) error {
	present := []hostbackup.FileRoot{}
	for _, root := range state.Roots {
		if _, errorValue := os.Stat(root.Path); errorValue == nil {
			present = append(present, root)
		}
	}
	state.referenced = map[string]bool{}
	if errorValue := state.keep(ctx, true, nil, func(output io.Writer, snapshots hostbackup.Snapshotter) error {
		_, errorValue := hostbackup.WriteFiles(output, present, snapshots)
		return errorValue
	}); errorValue != nil {
		return errorValue
	}
	state.lastFull = time.Now()
	return state.pruneBehindTheFullEntry(ctx)
}

func (state *following) keep(ctx context.Context, isFull bool, removed []string, write func(io.Writer, hostbackup.Snapshotter) error) error {
	scratchPath, errorValue := os.MkdirTemp(state.ScratchDirectoryPath, "file-journal-")
	if errorValue != nil {
		return errorValue
	}
	defer os.RemoveAll(scratchPath)
	kept, errorValue := state.uploader.KeepContent(func(output io.Writer) error {
		return write(output, hostbackup.SQLiteSnapshots{ScratchDirectoryPath: scratchPath})
	})
	if errorValue != nil {
		return fmt.Errorf("keeping the company's changed files: %w", errorValue)
	}
	roles := []string{}
	for _, root := range state.Roots {
		roles = append(roles, root.Role)
	}
	entry := Entry{Version: entryVersion, Sequence: state.next, CreatedAt: time.Now().UTC(), IsFull: isFull, Roles: roles, Removed: removed, Chunks: kept.Chunks, Size: kept.Size, SHA256: kept.SHA256}
	if errorValue := state.Journal.Keep(ctx, entry); errorValue != nil {
		return errorValue
	}
	state.next++
	for _, identifier := range kept.Chunks {
		state.referenced[identifier] = true
	}
	return nil
}

func (state *following) pruneBehindTheFullEntry(ctx context.Context) error {
	fullSequence := state.next - 1
	sequences, errorValue := state.Journal.Sequences(ctx)
	if errorValue != nil {
		return errorValue
	}
	stale := []string{}
	for _, sequence := range sequences {
		if sequence < fullSequence {
			stale = append(stale, state.Journal.folder()+fmt.Sprintf(entryNameLayout, sequence))
		}
	}
	if errorValue := removeInBatches(ctx, state.Journal.Destination.Store, stale); errorValue != nil {
		return errorValue
	}
	chunkFolder := state.Journal.Destination.CompanyID + "/" + fileChunkFolder + "/"
	listed, errorValue := state.Journal.Destination.Store.List(ctx, chunkFolder)
	if errorValue != nil {
		return errorValue
	}
	unreferenced := []string{}
	unreferencedPaths := []string{}
	for _, object := range listed {
		if !state.referenced[object.Name] {
			unreferenced = append(unreferenced, object.Name)
			unreferencedPaths = append(unreferencedPaths, chunkFolder+object.Name)
		}
	}
	state.uploader.Forget(unreferenced)
	return removeInBatches(ctx, state.Journal.Destination.Store, unreferencedPaths)
}

const removeBatchSize = 1000

func removeInBatches(ctx context.Context, store offsite.Store, paths []string) error {
	for start := 0; start < len(paths); start += removeBatchSize {
		if errorValue := store.Remove(ctx, paths[start:min(start+removeBatchSize, len(paths))]); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
