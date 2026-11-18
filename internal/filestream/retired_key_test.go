package filestream

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

func keepAJournal(t *testing.T, follower Follower, changes int) {
	t.Helper()
	state, errorValue := follower.start(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for number := range changes {
		keepOneChange(t, state, number)
	}
}

func TestABoxTheCompanyIsReKeyedToStartsItsOwnJournalOverTheOneARetiredKeySealed(t *testing.T) {
	kept := &keptObjects{objects: map[string][]byte{}}
	keepAJournal(t, aFollower(t, kept), 3)
	reKeyed := aFollower(t, kept)

	keepAJournal(t, reKeyed, 2)
	cache := offsite.ChunkCache{DirectoryPath: filepath.Join(t.TempDir(), "file-chunks")}
	sequences := []int64{}
	replayed, errorValue := reKeyed.Journal.Replay(context.Background(), reKeyed.Keyring, cache, func(entry Entry, contents io.Reader) error {
		sequences = append(sequences, entry.Sequence)
		_, errorValue := io.Copy(io.Discard, contents)
		return errorValue
	})

	if errorValue != nil {
		t.Fatalf("the journal the retired key sealed stopped the re-keyed box: %v", errorValue)
	}
	if replayed != 3 || sequences[0] != 5 {
		t.Fatalf("replayed entries %v, want the re-keyed box's full entry 5 and its two changes", sequences)
	}
}

func TestAnEntrySealedToTheCurrentKeyThatThisBoxCannotOpenStaysAnError(t *testing.T) {
	kept := &keptObjects{objects: map[string][]byte{}}
	serving := aFollower(t, kept)
	keepAJournal(t, serving, 2)
	directory := t.TempDir()
	stranger, errorValue := box.LoadOrCreateIdentity(filepath.Join(directory, "identity.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	withoutTheKey, errorValue := box.KeyringFor(stranger, directory, companyID, serving.Journal.Destination.SealingKey, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	cache := offsite.ChunkCache{DirectoryPath: filepath.Join(t.TempDir(), "file-chunks")}

	_, errorValue = serving.Journal.Replay(context.Background(), withoutTheKey, cache, func(Entry, io.Reader) error { return nil })

	if !errors.Is(errorValue, box.ErrCompanyKeyNotHeld) {
		t.Fatalf("a journal sealed to the company's current key was read without it: %v", errorValue)
	}
}
