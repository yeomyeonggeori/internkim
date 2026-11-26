package filestream

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

type fullEntryInFlight struct {
	sequence     int64
	startedAt    time.Time
	kept         chan fullEntryKept
	chunksSince  map[string]bool
	entriesSince int
	markers      []string

	isKeepingDroppedChanges bool
}

type fullEntryKept struct {
	content    offsite.KeptContent
	errorValue error
}

func (state *following) isAFullEntryDue() bool {
	return time.Since(state.lastFull) >= fullEntryEvery || state.entriesKept > fullEntryAfter
}

func (state *following) fullEntryKept() <-chan fullEntryKept {
	if state.fullEntry == nil {
		return nil
	}
	return state.fullEntry.kept
}

func (state *following) startFullEntry(ctx context.Context) error {
	uploader, errorValue := offsite.StartSnapshot(ctx, state.Journal.chunkDestination(), state.Keyring)
	if errorValue != nil {
		return errorValue
	}
	present := []hostbackup.FileRoot{}
	for _, root := range state.Roots {
		if _, errorValue := os.Stat(root.Path); errorValue == nil {
			present = append(present, root)
		}
	}
	inFlight := &fullEntryInFlight{
		sequence:    state.next,
		startedAt:   time.Now().UTC(),
		kept:        make(chan fullEntryKept, 1),
		chunksSince: map[string]bool{},
	}
	state.next++
	state.fullEntry = inFlight
	follower := state.Follower
	log.Printf("keeping the whole tree as file journal entry %d while changes go on being kept", inFlight.sequence)
	go func() {
		content, errorValue := follower.keepContent(uploader, func(output io.Writer, snapshots hostbackup.Snapshotter) error {
			_, errorValue := hostbackup.WriteFiles(output, present, snapshots)
			return errorValue
		})
		inFlight.kept <- fullEntryKept{content: content, errorValue: errorValue}
	}()
	return nil
}

func (state *following) keepTheWholeTreeAgain(ctx context.Context, markers []string) error {
	if state.fullEntry != nil {
		state.isFullEntryOwed = true
		state.owedMarkers = append(state.owedMarkers, markers...)
		return nil
	}
	if errorValue := state.startFullEntry(ctx); errorValue != nil {
		return errorValue
	}
	state.fullEntry.isKeepingDroppedChanges = true
	state.fullEntry.markers = append(state.fullEntry.markers, markers...)
	return nil
}

func (state *following) holdsUntilDroppedChangesAreKept(markers []string) bool {
	if state.isFullEntryOwed {
		state.owedMarkers = append(state.owedMarkers, markers...)
		return true
	}
	if state.fullEntry == nil || !state.fullEntry.isKeepingDroppedChanges {
		return false
	}
	state.fullEntry.markers = append(state.fullEntry.markers, markers...)
	return true
}

func (state *following) landFullEntry(ctx context.Context, kept fullEntryKept) error {
	inFlight := state.fullEntry
	state.fullEntry = nil
	if kept.errorValue != nil {
		return fmt.Errorf("keeping the company's whole tree: %w", kept.errorValue)
	}
	entry := Entry{
		Version:   entryVersion,
		Sequence:  inFlight.sequence,
		CreatedAt: inFlight.startedAt,
		IsFull:    true,
		Roles:     state.roles(),
		Chunks:    kept.content.Chunks,
		Size:      kept.content.Size,
		SHA256:    kept.content.SHA256,
	}
	if errorValue := state.Journal.Keep(ctx, entry); errorValue != nil {
		return errorValue
	}
	state.referenced = inFlight.chunksSince
	for _, identifier := range kept.content.Chunks {
		state.referenced[identifier] = true
	}
	state.lastFull = inFlight.startedAt
	state.entriesKept = 1 + inFlight.entriesSince
	if errorValue := state.pruneBehindTheFullEntry(ctx, inFlight.sequence); errorValue != nil {
		return errorValue
	}
	log.Printf("kept the whole tree as file journal entry %d in %s", inFlight.sequence, time.Since(inFlight.startedAt).Round(time.Second))
	state.Barrier.release(inFlight.markers)
	if !state.isFullEntryOwed {
		return nil
	}
	owedMarkers := state.owedMarkers
	state.isFullEntryOwed = false
	state.owedMarkers = nil
	return state.keepTheWholeTreeAgain(ctx, owedMarkers)
}

func (state *following) pruneBehindTheFullEntry(ctx context.Context, fullSequence int64) error {
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
