package companyhost

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

type journalEntry struct {
	isFull  bool
	removed []string
	files   map[string]string
}

func tarOf(t *testing.T, files map[string]string) io.Reader {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for name, contents := range files {
		if errorValue := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(contents)), Typeflag: tar.TypeReg}); errorValue != nil {
			t.Fatal(errorValue)
		}
		writer.Write([]byte(contents))
	}
	writer.Close()
	return &buffer
}

func TestATakeOverReplaysTheFileJournalOverTheSnapshot(t *testing.T) {
	workspace := t.TempDir()
	os.WriteFile(filepath.Join(workspace, "from-the-snapshot.txt"), []byte("stale"), 0o644)
	entries := []journalEntry{
		{isFull: true, files: map[string]string{"workspace/report.txt": "first", "workspace/doomed.txt": "soon gone"}},
		{removed: []string{"workspace/doomed.txt"}, files: map[string]string{"workspace/report.txt": "second", "workspace/last.txt": "written last"}},
	}
	var extracted hostbackup.ExtractionReport
	replay := func(apply FileJournalApplier) (int, error) {
		for _, entry := range entries {
			if errorValue := apply(entry.isFull, entry.removed, tarOf(t, entry.files)); errorValue != nil {
				return 0, errorValue
			}
		}
		return len(entries), nil
	}
	if _, errorValue := replayTheFileJournal(replay, map[string]string{"workspace": workspace}, &extracted); errorValue != nil {
		t.Fatal(errorValue)
	}
	for name, want := range map[string]string{"report.txt": "second", "last.txt": "written last"} {
		got, errorValue := os.ReadFile(filepath.Join(workspace, name))
		if errorValue != nil || string(got) != want {
			t.Errorf("%s holds %q (%v), want %q", name, got, errorValue, want)
		}
	}
	for _, gone := range []string{"doomed.txt", "from-the-snapshot.txt"} {
		if _, errorValue := os.Stat(filepath.Join(workspace, gone)); !os.IsNotExist(errorValue) {
			t.Errorf("%s survived the replay", gone)
		}
	}
}

func TestATakeOverOwnsOnlyWhatTheFileJournalLeft(t *testing.T) {
	workspace := resolvedTemporaryDirectory(t)
	entries := []journalEntry{
		{files: map[string]string{"workspace/memory.db-wal": "checkpointed soon", "workspace/notes/draft.txt": "dropped with its folder", "workspace/kept.txt": "stays"}},
		{removed: []string{"workspace/memory.db-wal", "workspace/notes"}},
	}
	extracted := replayed(t, workspace, entries)
	if paths := pathsOf(extracted.Unresolved); len(paths) != 1 || paths[0] != filepath.Join(workspace, "kept.txt") {
		t.Errorf("left to own once the people exist: %v, want only kept.txt", paths)
	}
	if _, errorValue := ownWhatThePeopleOwn(extracted.Unresolved); errorValue != nil {
		t.Errorf("owning what the journal left: %v", errorValue)
	}
}

func TestAFullFileJournalEntryLeavesNothingEarlierToOwn(t *testing.T) {
	workspace := resolvedTemporaryDirectory(t)
	entries := []journalEntry{
		{files: map[string]string{"workspace/before.txt": "emptied"}},
		{isFull: true, files: map[string]string{"workspace/after.txt": "stays"}},
	}
	extracted := replayed(t, workspace, entries)
	if paths := pathsOf(extracted.Unresolved); len(paths) != 1 || paths[0] != filepath.Join(workspace, "after.txt") {
		t.Errorf("left to own once the people exist: %v, want only after.txt", paths)
	}
}

func replayed(t *testing.T, workspace string, entries []journalEntry) hostbackup.ExtractionReport {
	t.Helper()
	replay := func(apply FileJournalApplier) (int, error) {
		for index, entry := range entries {
			if errorValue := apply(entry.isFull, entry.removed, tarOf(t, entry.files)); errorValue != nil {
				return index, errorValue
			}
		}
		return len(entries), nil
	}
	var extracted hostbackup.ExtractionReport
	if _, errorValue := replayTheFileJournal(replay, map[string]string{"workspace": workspace}, &extracted); errorValue != nil {
		t.Fatal(errorValue)
	}
	return extracted
}

func resolvedTemporaryDirectory(t *testing.T) string {
	t.Helper()
	directory, errorValue := filepath.EvalSymlinks(t.TempDir())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return directory
}

func pathsOf(ownerships []hostbackup.Ownership) []string {
	paths := []string{}
	for _, ownership := range ownerships {
		paths = append(paths, ownership.Path)
	}
	return paths
}

func TestTheFileJournalNeverRemovesOutsideItsRoot(t *testing.T) {
	for _, name := range []string{"workspace/../escape", "workspace/", "nowhere/file", "workspace"} {
		if _, errorValue := removeArchived(map[string]string{"workspace": t.TempDir()}, name); errorValue == nil {
			t.Errorf("%q was removed", name)
		}
	}
}
