package hostbackup

import (
	"archive/tar"
	"bytes"
	"database/sql"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, contents := range files {
		path := filepath.Join(root, name)
		if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
		if errorValue := os.WriteFile(path, []byte(contents), 0o640); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func archiveAndExtract(t *testing.T, roots []FileRoot, destinations map[string]string) (FilesReport, ExtractionReport) {
	t.Helper()
	var archived bytes.Buffer
	written, errorValue := WriteFiles(&archived, roots, SQLiteSnapshots{ScratchDirectoryPath: t.TempDir()})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	extracted, errorValue := ExtractFiles(&archived, destinations)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return written, extracted
}

func TestFilesComeBackUnderTheRootsTheyWereTakenFrom(t *testing.T) {
	source := t.TempDir()
	writeTree(t, source, map[string]string{
		"private/people/a/notes.md":    "a's notes",
		"private/people/a/tmp/scratch": "rebuildable",
		"shared/cache/uv/wheel":        "rebuildable",
		"shared/public/deck.pdf":       "deck",
	})
	if errorValue := os.Symlink("private/people/a/notes.md", filepath.Join(source, "link")); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.Chmod(filepath.Join(source, "shared/public"), 0o2775); errorValue != nil {
		t.Fatal(errorValue)
	}
	destination := filepath.Join(t.TempDir(), "workspace")
	roots := []FileRoot{{Role: "workspace", Path: source, Excluded: []string{"shared/cache", "private/people/*/tmp"}}}
	written, extracted := archiveAndExtract(t, roots, map[string]string{"workspace": destination})

	if written.Files != 2 || extracted.Files != 2 {
		t.Errorf("wrote %d and restored %d files; the two outside the caches are wanted", written.Files, extracted.Files)
	}
	if document, _ := os.ReadFile(filepath.Join(destination, "private/people/a/notes.md")); string(document) != "a's notes" {
		t.Errorf("notes.md came back as %q", document)
	}
	for _, left := range []string{"shared/cache", "private/people/a/tmp"} {
		if _, errorValue := os.Lstat(filepath.Join(destination, left)); errorValue == nil {
			t.Errorf("%s was restored although it is excluded", left)
		}
	}
	if target, _ := os.Readlink(filepath.Join(destination, "link")); target != "private/people/a/notes.md" {
		t.Errorf("the symlink points at %q", target)
	}
	original, _ := os.Stat(filepath.Join(source, "shared/public"))
	restored, _ := os.Stat(filepath.Join(destination, "shared/public"))
	if restored.Mode() != original.Mode() {
		t.Errorf("shared/public came back as %v, not %v", restored.Mode(), original.Mode())
	}
}

func TestOwnersAreRecordedByNameAndNotByNumber(t *testing.T) {
	source := t.TempDir()
	writeTree(t, source, map[string]string{"file": "x"})
	var archived bytes.Buffer
	if _, errorValue := WriteFiles(&archived, []FileRoot{{Role: "state", Path: source}}, SQLiteSnapshots{ScratchDirectoryPath: t.TempDir()}); errorValue != nil {
		t.Fatal(errorValue)
	}
	current, _ := user.Current()
	reader := tar.NewReader(&archived)
	for {
		header, errorValue := reader.Next()
		if errorValue == io.EOF {
			break
		}
		if header.Uname != current.Username || header.Gname == "" {
			t.Errorf("%s records owner %q:%q, not the account's name", header.Name, header.Uname, header.Gname)
		}
	}
}

func TestAnOwnerThisComputerLacksIsReportedAndNotGuessedFromItsNumber(t *testing.T) {
	current, _ := user.Current()
	group, _ := user.LookupGroupId(current.Gid)
	var archived bytes.Buffer
	writer := tar.NewWriter(&archived)
	writer.WriteHeader(&tar.Header{Name: "workspace/", Typeflag: tar.TypeDir, Mode: 0o755, Uname: current.Username, Gname: group.Name})
	writer.WriteHeader(&tar.Header{
		Name: "workspace/private.md", Typeflag: tar.TypeReg, Mode: 0o600, Size: 1,
		Uname: "bc_person_nobody_here", Gname: "bc_person_nobody_here", Uid: os.Getuid(), Gid: os.Getgid(),
	})
	writer.Write([]byte("x"))
	writer.Close()
	destination := filepath.Join(t.TempDir(), "workspace")
	extracted, errorValue := ExtractFiles(&archived, map[string]string{"workspace": destination})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(extracted.Unresolved) != 1 || extracted.Unresolved[0].UserName != "bc_person_nobody_here" {
		t.Fatalf("the unknown owner was not reported: %+v", extracted.Unresolved)
	}
}

func TestAnEntryCannotBeWrittenThroughASymlinkTheArchivePlanted(t *testing.T) {
	outside := t.TempDir()
	var archived bytes.Buffer
	writer := tar.NewWriter(&archived)
	writer.WriteHeader(&tar.Header{Name: "workspace/", Typeflag: tar.TypeDir, Mode: 0o755})
	writer.WriteHeader(&tar.Header{Name: "workspace/door", Typeflag: tar.TypeSymlink, Linkname: outside})
	writer.WriteHeader(&tar.Header{Name: "workspace/door/planted", Typeflag: tar.TypeReg, Mode: 0o600, Size: 1})
	writer.Write([]byte("x"))
	writer.Close()
	_, errorValue := ExtractFiles(&archived, map[string]string{"workspace": filepath.Join(t.TempDir(), "workspace")})
	if errorValue == nil {
		t.Fatal("an entry was written through a symlink")
	}
	if _, statError := os.Stat(filepath.Join(outside, "planted")); statError == nil {
		t.Fatal("the planted file landed outside the root")
	}
}

func TestAnEntryThatClimbsOutOfItsRootIsRefused(t *testing.T) {
	var archived bytes.Buffer
	writer := tar.NewWriter(&archived)
	writer.WriteHeader(&tar.Header{Name: "workspace/../../escape", Typeflag: tar.TypeReg, Mode: 0o600, Size: 1})
	writer.Write([]byte("x"))
	writer.Close()
	_, errorValue := ExtractFiles(&archived, map[string]string{"workspace": filepath.Join(t.TempDir(), "workspace")})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "outside") {
		t.Fatalf("an entry outside every root was accepted: %v", errorValue)
	}
}

func TestAnEntryUnderARootTheBackupDoesNotListIsRefused(t *testing.T) {
	var archived bytes.Buffer
	writer := tar.NewWriter(&archived)
	writer.WriteHeader(&tar.Header{Name: "elsewhere/file", Typeflag: tar.TypeReg, Mode: 0o600, Size: 1})
	writer.Write([]byte("x"))
	writer.Close()
	if _, errorValue := ExtractFiles(&archived, map[string]string{"workspace": t.TempDir()}); errorValue == nil {
		t.Fatal("an entry under an unknown root was accepted")
	}
}

func TestALiveSQLiteDatabaseIsTakenWholeWithWhatItsLogHolds(t *testing.T) {
	source := t.TempDir()
	databasePath := filepath.Join(source, "internkim.sqlite")
	database, errorValue := sql.Open("sqlite", "file:"+databasePath+"?_pragma=journal_mode(WAL)")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	for _, statement := range []string{
		"PRAGMA wal_autocheckpoint=0",
		"CREATE TABLE profile (name TEXT)",
		"INSERT INTO profile VALUES ('이샘플')",
	} {
		if _, errorValue := database.Exec(statement); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if information, errorValue := os.Stat(databasePath + "-wal"); errorValue != nil || information.Size() == 0 {
		t.Fatalf("the test needs the row to still be in the write-ahead log: %v", errorValue)
	}
	destination := filepath.Join(t.TempDir(), "state")
	archiveAndExtract(t, []FileRoot{{Role: "state", Path: source}}, map[string]string{"state": destination})

	if _, errorValue := os.Stat(filepath.Join(destination, "internkim.sqlite-wal")); errorValue == nil {
		t.Error("the write-ahead log was archived beside a snapshot that already holds it")
	}
	restored, errorValue := sql.Open("sqlite", "file:"+filepath.Join(destination, "internkim.sqlite")+"?mode=ro")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer restored.Close()
	var name string
	if errorValue := restored.QueryRow("SELECT name FROM profile").Scan(&name); errorValue != nil || name != "이샘플" {
		t.Fatalf("the restored database lost the row its log held: %q %v", name, errorValue)
	}
}

func TestARootThatIsASymlinkIsFollowedOnBothSides(t *testing.T) {
	sourceTarget := t.TempDir()
	writeTree(t, sourceTarget, map[string]string{"shared/public/deck.pdf": "deck"})
	sourceLink := filepath.Join(t.TempDir(), "workspace")
	os.Symlink(sourceTarget, sourceLink)
	destinationTarget := t.TempDir()
	destinationLink := filepath.Join(t.TempDir(), "workspace")
	os.Symlink(destinationTarget, destinationLink)

	archiveAndExtract(t, []FileRoot{{Role: "workspace", Path: sourceLink}}, map[string]string{"workspace": destinationLink})

	if document, _ := os.ReadFile(filepath.Join(destinationTarget, "shared/public/deck.pdf")); string(document) != "deck" {
		t.Errorf("the file did not land behind the destination's symlink: %q", document)
	}
	if information, errorValue := os.Lstat(destinationLink); errorValue != nil || information.Mode()&os.ModeSymlink == 0 {
		t.Error("the destination's symlink was replaced")
	}
}
