package capabilityd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

func TestClassifyMattermostUploadChannelDM(t *testing.T) {
	kind, ok := classifyMattermostUploadChannel("D", "")
	if !ok || kind != "dm" {
		t.Fatalf("expected dm/true, got %q/%v", kind, ok)
	}
}

func TestClassifyMattermostUploadChannelTownSquare(t *testing.T) {
	kind, ok := classifyMattermostUploadChannel("O", mattermostdefaults.TownSquareChannelName)
	if !ok || kind != "public" {
		t.Fatalf("expected public/true, got %q/%v", kind, ok)
	}
}

func TestClassifyMattermostUploadChannelOffTopic(t *testing.T) {
	kind, ok := classifyMattermostUploadChannel("O", mattermostdefaults.OffTopicChannelName)
	if !ok || kind != "public" {
		t.Fatalf("expected public/true, got %q/%v", kind, ok)
	}
}

func TestClassifyMattermostUploadChannelFlowExcluded(t *testing.T) {
	_, ok := classifyMattermostUploadChannel("O", mattermostdefaults.FlowChannelName)
	if ok {
		t.Fatal("expected flow channel to be excluded")
	}
}

func TestClassifyMattermostUploadChannelCalendarExcluded(t *testing.T) {
	_, ok := classifyMattermostUploadChannel("O", mattermostdefaults.CalendarChannelName)
	if ok {
		t.Fatal("expected calendar channel to be excluded")
	}
}

func TestClassifyMattermostUploadChannelAttendanceExcluded(t *testing.T) {
	_, ok := classifyMattermostUploadChannel("O", mattermostdefaults.AttendanceChannelName)
	if ok {
		t.Fatal("expected attendance channel to be excluded")
	}
}

func TestClassifyMattermostUploadChannelOtherPublicIsCircleCandidate(t *testing.T) {
	kind, ok := classifyMattermostUploadChannel("O", "some-other-channel")
	if !ok || kind != "circle-candidate" {
		t.Fatalf("expected circle-candidate/true for other public channel, got %q/%v", kind, ok)
	}
}

func TestClassifyMattermostUploadChannelPrivateIsCircleCandidate(t *testing.T) {
	kind, ok := classifyMattermostUploadChannel("P", "circle-engineering")
	if !ok || kind != "circle-candidate" {
		t.Fatalf("expected circle-candidate/true for private channel, got %q/%v", kind, ok)
	}
}

func TestClassifyMattermostUploadChannelGroupUnknown(t *testing.T) {
	_, ok := classifyMattermostUploadChannel("G", "some-group")
	if ok {
		t.Fatal("expected group channel to not be routable")
	}
}

func TestMattermostImportRecordStoreRoundtrip(t *testing.T) {
	workspacePath := t.TempDir()
	service := Service{Configuration: Configuration{BlueclawWorkspacePath: workspacePath}}

	originalRecords := []mattermostImportRecord{
		{FileID: "file-1", PostID: "post-1", ChannelID: "channel-1"},
		{FileID: "file-2", PostID: "post-2", ChannelID: "channel-2"},
	}

	if errorValue := service.saveMattermostImportRecords(originalRecords); errorValue != nil {
		t.Fatalf("save failed: %v", errorValue)
	}

	loadedRecords := service.loadMattermostImportRecords()
	if len(loadedRecords) != len(originalRecords) {
		t.Fatalf("expected %d records, got %d", len(originalRecords), len(loadedRecords))
	}
	for index, record := range originalRecords {
		loaded := loadedRecords[index]
		if loaded.FileID != record.FileID || loaded.PostID != record.PostID || loaded.ChannelID != record.ChannelID {
			t.Fatalf("record %d mismatch: expected %+v, got %+v", index, record, loaded)
		}
	}
}

func TestMattermostImportRecordStoreIdempotency(t *testing.T) {
	workspacePath := t.TempDir()
	service := Service{Configuration: Configuration{BlueclawWorkspacePath: workspacePath}}

	records := []mattermostImportRecord{
		{FileID: "file-1", PostID: "post-1", ChannelID: "channel-1"},
	}
	if errorValue := service.saveMattermostImportRecords(records); errorValue != nil {
		t.Fatalf("save failed: %v", errorValue)
	}

	loadedOnce := service.loadMattermostImportRecords()
	fileIDs := importedFileIDSet(loadedOnce)
	if !fileIDs["file-1"] {
		t.Fatal("expected file-1 to be detected as already recorded")
	}
	if fileIDs["file-2"] {
		t.Fatal("expected file-2 to not be in the set")
	}
}

func TestMattermostImportStoreFileLocation(t *testing.T) {
	workspacePath := t.TempDir()
	service := Service{Configuration: Configuration{BlueclawWorkspacePath: workspacePath}}

	records := []mattermostImportRecord{
		{FileID: "file-x", PostID: "post-x", ChannelID: "channel-x"},
	}
	if errorValue := service.saveMattermostImportRecords(records); errorValue != nil {
		t.Fatalf("save failed: %v", errorValue)
	}

	expectedPath := filepath.Join(workspacePath, ".blueclaw", "mattermost-imports.json")
	if _, errorValue := os.Stat(expectedPath); errorValue != nil {
		t.Fatalf("expected store at %s, got error: %v", expectedPath, errorValue)
	}

	document, errorValue := os.ReadFile(expectedPath)
	if errorValue != nil {
		t.Fatalf("read failed: %v", errorValue)
	}
	var store mattermostImportRecordStore
	if errorValue := json.Unmarshal(document, &store); errorValue != nil {
		t.Fatalf("unmarshal failed: %v", errorValue)
	}
	if len(store.Records) != 1 || store.Records[0].FileID != "file-x" {
		t.Fatalf("unexpected store contents: %+v", store)
	}
}

func TestExistingAttachmentByContentMatchesIdenticalContent(t *testing.T) {
	directory := t.TempDir()
	content := []byte("<html><body>deck</body></html>")
	if errorValue := os.WriteFile(filepath.Join(directory, "ir-deck-v3.html"), content, 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	filename, isFound := existingAttachmentByContent(directory, content)
	if !isFound || filename != "ir-deck-v3.html" {
		t.Fatalf("expected to reuse identical file, got %q found=%v", filename, isFound)
	}

	if _, isFound := existingAttachmentByContent(directory, []byte("different content")); isFound {
		t.Fatal("expected no reuse for different content")
	}
}

func TestExistingAttachmentByContentIgnoresSameSizeDifferentContent(t *testing.T) {
	directory := t.TempDir()
	if errorValue := os.WriteFile(filepath.Join(directory, "a.txt"), []byte("aaaa"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, isFound := existingAttachmentByContent(directory, []byte("bbbb")); isFound {
		t.Fatal("expected no reuse when sizes match but content differs")
	}
}
