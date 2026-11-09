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
	if errorValue := replayTheFileJournal(replay, map[string]string{"workspace": workspace}, &extracted, io.Discard); errorValue != nil {
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

func TestTheFileJournalNeverRemovesOutsideItsRoot(t *testing.T) {
	for _, name := range []string{"workspace/../escape", "workspace/", "nowhere/file", "workspace"} {
		if errorValue := removeArchived(map[string]string{"workspace": t.TempDir()}, name); errorValue == nil {
			t.Errorf("%q was removed", name)
		}
	}
}
