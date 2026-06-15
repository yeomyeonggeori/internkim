package capabilityd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		{FileID: "file-1", PostID: "post-1", ChannelID: "channel-1", ImportedAt: "2026-01-01T00:00:00Z"},
		{FileID: "file-2", PostID: "post-2", ChannelID: "channel-2", ImportedAt: "2026-01-02T00:00:00Z"},
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
		if loaded.FileID != record.FileID || loaded.PostID != record.PostID || loaded.ChannelID != record.ChannelID || loaded.ImportedAt != record.ImportedAt {
			t.Fatalf("record %d mismatch: expected %+v, got %+v", index, record, loaded)
		}
	}
}

func TestMattermostImportRecordStoreIdempotency(t *testing.T) {
	workspacePath := t.TempDir()
	service := Service{Configuration: Configuration{BlueclawWorkspacePath: workspacePath}}

	records := []mattermostImportRecord{
		{FileID: "file-1", PostID: "post-1", ChannelID: "channel-1", ImportedAt: "2026-01-01T00:00:00Z"},
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
		{FileID: "file-x", PostID: "post-x", ChannelID: "channel-x", ImportedAt: "2026-01-01T00:00:00Z"},
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

func TestMattermostImportRecordExpiredRetentionWindow(t *testing.T) {
	now := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)

	thirtyOneDaysAgo := now.Add(-31 * 24 * time.Hour).Format(time.RFC3339)
	if !mattermostImportRecordExpired(thirtyOneDaysAgo, now) {
		t.Fatal("expected record 31 days old to be expired")
	}

	twentyNineDaysAgo := now.Add(-29 * 24 * time.Hour).Format(time.RFC3339)
	if mattermostImportRecordExpired(twentyNineDaysAgo, now) {
		t.Fatal("expected record 29 days old to not be expired")
	}

	exactly30DaysAgo := now.Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	if mattermostImportRecordExpired(exactly30DaysAgo, now) {
		t.Fatal("expected record exactly 30 days old to not be expired (boundary)")
	}
}

func TestMattermostImportRecordExpiredInvalidTimestamp(t *testing.T) {
	now := time.Now().UTC()
	if mattermostImportRecordExpired("not-a-date", now) {
		t.Fatal("expected invalid timestamp to not be considered expired")
	}
	if mattermostImportRecordExpired("", now) {
		t.Fatal("expected empty timestamp to not be considered expired")
	}
}

func TestMattermostImportRecordExpiredFuture(t *testing.T) {
	now := time.Now().UTC()
	future := now.Add(24 * time.Hour).Format(time.RFC3339)
	if mattermostImportRecordExpired(future, now) {
		t.Fatal("expected future timestamp to not be expired")
	}
}
