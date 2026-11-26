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
	fullEntryAfter = 2000
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
	uploader        *offsite.Snapshot
	next            int64
	referenced      map[string]bool
	lastFull        time.Time
	entriesKept     int
	unchanged       keptStamps
	fullEntry       *fullEntryInFlight
	isFullEntryOwed bool
	owedMarkers     []string
}

func (follower Follower) Run(ctx context.Context) error {
	watcher, errorValue := NewWatcher(follower.Roots)
	if errorValue != nil {
		return errorValue
	}
	defer watcher.Close()
	state, errorValue := follower.start(ctx)
	if errorValue != nil {
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

func (follower Follower) start(ctx context.Context) (*following, error) {
	uploader, errorValue := offsite.StartSnapshot(ctx, follower.Journal.chunkDestination(), follower.Keyring)
	if errorValue != nil {
		return nil, errorValue
	}
	sequences, errorValue := follower.Journal.Sequences(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	state := &following{Follower: follower, uploader: uploader, next: 1, entriesKept: len(sequences), unchanged: keptStamps{}, referenced: map[string]bool{}}
	if len(sequences) > 0 {
		state.next = slices.Max(sequences) + 1
	}
	if errorValue := state.catchUp(ctx); errorValue != nil {
		return nil, errorValue
	}
	return state, nil
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
		case kept := <-state.fullEntryKept():
			if errorValue := state.landFullEntry(ctx, kept); errorValue != nil {
				return errorValue
			}
			continue
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
	answered := gathered.markers
	switch {
	case gathered.isOverflowed:
		log.Printf("the kernel dropped file changes it could not queue, so the whole tree is kept again")
		if errorValue := state.keepTheWholeTreeAgain(ctx, gathered.markers); errorValue != nil {
			return errorValue
		}
		answered = nil
	case state.holdsUntilDroppedChangesAreKept(gathered.markers):
		answered = nil
	case state.fullEntry == nil && state.isAFullEntryDue():
		if errorValue := state.startFullEntry(ctx); errorValue != nil {
			return errorValue
		}
	}
	if len(gathered.changes) > 0 {
		if errorValue := state.keepChanges(ctx, gathered.changes); errorValue != nil {
			return errorValue
		}
	}
	state.Barrier.release(answered)
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
	taken := keptStamps{}
	takenAt := time.Now()
	for _, name := range names {
		change := gathered[name]
		information, errorValue := os.Lstat(filepath.Join(change.Root.Path, filepath.FromSlash(change.Relative)))
		if errors.Is(errorValue, fs.ErrNotExist) {
			removed = append(removed, name)
			continue
		}
		if errorValue != nil {
			return errorValue
		}
		stamp := stampOf(information)
		if state.unchanged.holds(name, stamp) {
			continue
		}
		taken[name] = stamp
		present = append(present, hostbackup.ChangedEntry{Root: change.Root, Relative: change.Relative})
	}
	if len(present) == 0 && len(removed) == 0 {
		return nil
	}
	if errorValue := state.keepChangeEntry(ctx, removed, func(output io.Writer, snapshots hostbackup.Snapshotter) error {
		_, errorValue := hostbackup.WriteEntries(output, present, snapshots)
		return errorValue
	}); errorValue != nil {
		return errorValue
	}
	for _, name := range removed {
		delete(state.unchanged, name)
	}
	for name, stamp := range taken {
		state.unchanged.remember(name, stamp, takenAt)
	}
	return nil
}

func (follower Follower) keepContent(uploader *offsite.Snapshot, write func(io.Writer, hostbackup.Snapshotter) error) (offsite.KeptContent, error) {
	scratchPath, errorValue := os.MkdirTemp(follower.ScratchDirectoryPath, "file-journal-")
	if errorValue != nil {
		return offsite.KeptContent{}, errorValue
	}
	defer os.RemoveAll(scratchPath)
	return uploader.KeepContent(func(output io.Writer) error {
		return write(output, hostbackup.SQLiteSnapshots{ScratchDirectoryPath: scratchPath})
	})
}

func (follower Follower) roles() []string {
	roles := []string{}
	for _, root := range follower.Roots {
		roles = append(roles, root.Role)
	}
	return roles
}

func (state *following) keepChangeEntry(ctx context.Context, removed []string, write func(io.Writer, hostbackup.Snapshotter) error) error {
	kept, errorValue := state.keepContent(state.uploader, write)
	if errorValue != nil {
		return fmt.Errorf("keeping the company's changed files: %w", errorValue)
	}
	entry := Entry{Version: entryVersion, Sequence: state.next, CreatedAt: time.Now().UTC(), Roles: state.roles(), Removed: removed, Chunks: kept.Chunks, Size: kept.Size, SHA256: kept.SHA256}
	if errorValue := state.Journal.Keep(ctx, entry); errorValue != nil {
		return errorValue
	}
	state.next++
	state.entriesKept++
	for _, identifier := range kept.Chunks {
		state.referenced[identifier] = true
	}
	if state.fullEntry == nil {
		return nil
	}
	state.fullEntry.entriesSince++
	for _, identifier := range kept.Chunks {
		state.fullEntry.chunksSince[identifier] = true
	}
	return nil
}
