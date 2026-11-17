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
