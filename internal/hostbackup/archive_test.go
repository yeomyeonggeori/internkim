package hostbackup

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeExampleArchive(t *testing.T, members map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "example.tar")
	writer, errorValue := Create(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, name := range []string{DatabaseMemberName("blueclaw"), FilesMemberName} {
		contents, isWanted := members[name]
		if !isWanted {
			continue
		}
		if errorValue := writer.AddMember(name, func(output io.Writer) error {
			_, errorValue := io.WriteString(output, contents)
			return errorValue
		}); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if _, errorValue := writer.Finish(Manifest{CreatedAt: time.Now(), CompanyID: "company", PostgreSQLMajor: 17}); errorValue != nil {
		t.Fatal(errorValue)
	}
	return archivePath
}

func TestAnArchiveReadsBackWhatWasStreamedIntoIt(t *testing.T) {
	dump := strings.Repeat("dump-bytes-", 4000) + "tail"
	archivePath := writeExampleArchive(t, map[string]string{DatabaseMemberName("blueclaw"): dump, FilesMemberName: "files"})
	archive, errorValue := Open(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := archive.Verify(); errorValue != nil {
		t.Fatalf("an untouched archive was refused: %v", errorValue)
	}
	member, _ := archive.Manifest.Member(DatabaseMemberName("blueclaw"))
	if member.Size != int64(len(dump)) {
		t.Errorf("the manifest says %d bytes for a %d-byte dump", member.Size, len(dump))
	}
	var read bytes.Buffer
	if errorValue := archive.ReadMember(member.Name, func(input io.Reader) error {
		_, errorValue := io.Copy(&read, input)
		return errorValue
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if read.String() != dump {
		t.Error("the member read back is not the one streamed in")
	}
}

func TestAnArchiveIsAnOrdinaryTarThatOtherToolsList(t *testing.T) {
	archivePath := writeExampleArchive(t, map[string]string{DatabaseMemberName("blueclaw"): "dump", FilesMemberName: "files"})
	file, errorValue := os.Open(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer file.Close()
	reader := tar.NewReader(file)
	names := []string{}
	for {
		header, errorValue := reader.Next()
		if errorValue == io.EOF {
			break
		}
		if errorValue != nil {
			t.Fatalf("the archive is not a tar a reader can walk: %v", errorValue)
		}
		names = append(names, header.Name)
	}
	if strings.Join(names, " ") != "databases/blueclaw.dump files.tar manifest.json" {
		t.Errorf("the archive lists %v", names)
	}
}

func TestATamperedMemberIsRefusedByItsChecksum(t *testing.T) {
	archivePath := writeExampleArchive(t, map[string]string{DatabaseMemberName("blueclaw"): "the agent's ledger", FilesMemberName: "files"})
	document, errorValue := os.ReadFile(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	offset := bytes.Index(document, []byte("ledger"))
	document[offset] ^= 0xFF
	if errorValue := os.WriteFile(archivePath, document, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	archive, errorValue := Open(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	errorValue = archive.Verify()
	if errorValue == nil || !strings.Contains(errorValue.Error(), "databases/blueclaw.dump does not match") {
		t.Fatalf("a changed dump was not refused by name: %v", errorValue)
	}
}

func TestATruncatedArchiveIsRefused(t *testing.T) {
	archivePath := writeExampleArchive(t, map[string]string{DatabaseMemberName("blueclaw"): strings.Repeat("x", 5000), FilesMemberName: "files"})
	document, _ := os.ReadFile(archivePath)
	if errorValue := os.WriteFile(archivePath, document[:3000], 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := Open(archivePath); errorValue == nil {
		t.Fatal("an archive cut off part way was opened")
	}
}

func archiveWithManifest(t *testing.T, manifest map[string]any, extraMembers map[string]string) string {
	t.Helper()
	var held bytes.Buffer
	writer := tar.NewWriter(&held)
	for name, contents := range extraMembers {
		writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(contents)), Typeflag: tar.TypeReg})
		io.WriteString(writer, contents)
	}
	document, _ := json.Marshal(manifest)
	writer.WriteHeader(&tar.Header{Name: ManifestName, Mode: 0o600, Size: int64(len(document)), Typeflag: tar.TypeReg})
	writer.Write(document)
	writer.Close()
	archivePath := filepath.Join(t.TempDir(), "crafted.tar")
	if errorValue := os.WriteFile(archivePath, held.Bytes(), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return archivePath
}

func TestAnArchiveFromANewerFormatIsRefused(t *testing.T) {
	archivePath := archiveWithManifest(t, map[string]any{"format": FormatName, "formatVersion": FormatVersion + 1, "members": []any{}}, nil)
	_, errorValue := Open(archivePath)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "Upgrade internkim") {
		t.Fatalf("a format this version cannot read was opened: %v", errorValue)
	}
}

func TestAnArchiveOfAnotherFormatIsRefused(t *testing.T) {
	archivePath := archiveWithManifest(t, map[string]any{"format": "gitlab-backup", "formatVersion": 1}, nil)
	if _, errorValue := Open(archivePath); errorValue == nil {
		t.Fatal("an archive that names another format was opened")
	}
}

func TestAMemberTheManifestDoesNotListIsRefused(t *testing.T) {
	archivePath := archiveWithManifest(t,
		map[string]any{"format": FormatName, "formatVersion": FormatVersion, "members": []any{}},
		map[string]string{"smuggled.sh": "rm -rf /"})
	archive, errorValue := Open(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := archive.Verify(); errorValue == nil || !strings.Contains(errorValue.Error(), "smuggled.sh") {
		t.Fatalf("an unlisted member passed verification: %v", errorValue)
	}
}

func TestAListedMemberTheArchiveLacksIsRefused(t *testing.T) {
	archivePath := archiveWithManifest(t, map[string]any{
		"format": FormatName, "formatVersion": FormatVersion,
		"members": []any{map[string]any{"name": FilesMemberName, "size": 1, "sha256": "00"}},
	}, nil)
	archive, errorValue := Open(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := archive.Verify(); errorValue == nil || !strings.Contains(errorValue.Error(), "does not carry") {
		t.Fatalf("a missing member passed verification: %v", errorValue)
	}
}

func TestAMemberNameThatClimbsOutIsRefused(t *testing.T) {
	archivePath := archiveWithManifest(t, map[string]any{
		"format": FormatName, "formatVersion": FormatVersion,
		"members": []any{map[string]any{"name": "../escape", "size": 1, "sha256": "00"}},
	}, nil)
	if _, errorValue := Open(archivePath); errorValue == nil {
		t.Fatal("a manifest naming a member outside the archive was accepted")
	}
}

func TestDatabaseMembersNameTheirDatabase(t *testing.T) {
	database, isDatabase := DatabaseNamedBy(DatabaseMemberName("buzz"))
	if !isDatabase || database != "buzz" {
		t.Errorf("databases/buzz.dump named %q", database)
	}
	if _, isDatabase := DatabaseNamedBy("files.tar"); isDatabase {
		t.Error("files.tar was taken for a database")
	}
}
