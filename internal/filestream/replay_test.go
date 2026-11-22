package filestream

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

func TestAReplayHandsOverEveryEntrySinceTheFullOneInTheOrderTheyWereKept(t *testing.T) {
	const changes = 60
	kept := &keptObjects{objects: map[string][]byte{}}
	follower := aFollower(t, kept)
	state, errorValue := follower.start(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for number := range changes {
		keepOneChange(t, state, number)
	}
	cache := offsite.ChunkCache{DirectoryPath: filepath.Join(t.TempDir(), "file-chunks")}
	sequences := []int64{}
	sizes := int64(0)

	replayed, errorValue := follower.Journal.Replay(context.Background(), follower.Keyring, cache, func(entry Entry, contents io.Reader) error {
		sequences = append(sequences, entry.Sequence)
		read, errorValue := io.Copy(io.Discard, contents)
		sizes += read - entry.Size
		return errorValue
	})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if replayed != changes+1 || len(sequences) != changes+1 {
		t.Fatalf("replayed %d entries and applied %d, want the full entry and %d changes", replayed, len(sequences), changes)
	}
	for index, sequence := range sequences {
		if sequence != sequences[0]+int64(index) {
			t.Fatalf("entry %d was applied at position %d, out of the order it was kept in", sequence, index)
		}
	}
	if sizes != 0 {
		t.Fatalf("the entries' contents were %d bytes off what the journal recorded", sizes)
	}
}

func TestAMoveReplaysTheFileJournalThroughTheEntryTheOldBoxStoppedAt(t *testing.T) {
	kept := &keptObjects{objects: map[string][]byte{}}
	follower := aFollower(t, kept)
	state, errorValue := follower.start(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for number := range 5 {
		keepOneChange(t, state, number)
	}
	newest, hasEntries, errorValue := follower.Journal.Newest(context.Background())
	if errorValue != nil || !hasEntries {
		t.Fatalf("newest entry %d, held %v: %v", newest, hasEntries, errorValue)
	}
	cache := offsite.ChunkCache{DirectoryPath: filepath.Join(t.TempDir(), "file-chunks")}
	through := newest - 2
	applied := []int64{}

	_, errorValue = follower.Journal.ReplayThrough(context.Background(), follower.Keyring, cache, &through, func(entry Entry, contents io.Reader) error {
		applied = append(applied, entry.Sequence)
		_, errorValue := io.Copy(io.Discard, contents)
		return errorValue
	})

	if errorValue != nil || applied[len(applied)-1] != through {
		t.Fatalf("replaying through %d applied %v: %v", through, applied, errorValue)
	}
	missing := newest + 1
	if _, errorValue := follower.Journal.ReplayThrough(context.Background(), follower.Keyring, cache, &missing, func(Entry, io.Reader) error { return nil }); errorValue == nil {
		t.Fatal("a replay through an entry the store does not hold went ahead")
	}
}
