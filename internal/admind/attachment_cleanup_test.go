package admind

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAttachmentFixture(t *testing.T, path string, content string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, []byte(content), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestCleanupConversationAttachmentsDeduplicatesAndFlattens(t *testing.T) {
	workspaceRoot := t.TempDir()
	conversationDirectory := filepath.Join(workspaceRoot, "circles", "member", "inbox", "mattermost", "thread-1")
	writeAttachmentFixture(t, filepath.Join(conversationDirectory, "message-a", "deck.html"), "DECK")
	writeAttachmentFixture(t, filepath.Join(conversationDirectory, "message-b", "deck.html"), "DECK")
	writeAttachmentFixture(t, filepath.Join(conversationDirectory, "message-c", "notes.txt"), "NOTES")

	dryRun := attachmentCleanupReport{Actions: []attachmentCleanupAction{}}
	cleanupConversationAttachments(conversationDirectory, workspaceRoot, false, &dryRun)
	if dryRun.DuplicatesRemoved != 1 || dryRun.FilesFlattened != 2 {
		t.Fatalf("unexpected dry-run report: %+v", dryRun)
	}
	if _, errorValue := os.Stat(filepath.Join(conversationDirectory, "message-b", "deck.html")); errorValue != nil {
		t.Fatal("dry-run must not remove files")
	}

	applied := attachmentCleanupReport{Actions: []attachmentCleanupAction{}}
	cleanupConversationAttachments(conversationDirectory, workspaceRoot, true, &applied)
	if applied.DuplicatesRemoved != 1 || applied.FilesFlattened != 2 {
		t.Fatalf("unexpected apply report: %+v", applied)
	}

	if _, errorValue := os.Stat(filepath.Join(conversationDirectory, "deck.html")); errorValue != nil {
		t.Fatal("expected deck.html flattened into conversation directory")
	}
	if _, errorValue := os.Stat(filepath.Join(conversationDirectory, "notes.txt")); errorValue != nil {
		t.Fatal("expected notes.txt flattened into conversation directory")
	}
	for _, messageFolder := range []string{"message-a", "message-b", "message-c"} {
		if _, errorValue := os.Stat(filepath.Join(conversationDirectory, messageFolder)); !os.IsNotExist(errorValue) {
			t.Fatalf("expected empty message folder %s to be removed", messageFolder)
		}
	}
}

func TestCleanupConversationAttachmentsKeepsDistinctSameNameFiles(t *testing.T) {
	workspaceRoot := t.TempDir()
	conversationDirectory := filepath.Join(workspaceRoot, "circles", "member", "inbox", "mattermost", "thread-2")
	writeAttachmentFixture(t, filepath.Join(conversationDirectory, "message-a", "report.txt"), "FIRST")
	writeAttachmentFixture(t, filepath.Join(conversationDirectory, "message-b", "report.txt"), "SECOND")

	report := attachmentCleanupReport{Actions: []attachmentCleanupAction{}}
	cleanupConversationAttachments(conversationDirectory, workspaceRoot, true, &report)

	if report.DuplicatesRemoved != 0 || report.FilesFlattened != 2 {
		t.Fatalf("expected both distinct files kept, got %+v", report)
	}
	if _, errorValue := os.Stat(filepath.Join(conversationDirectory, "report.txt")); errorValue != nil {
		t.Fatal("expected first report.txt at top level")
	}
	if _, errorValue := os.Stat(filepath.Join(conversationDirectory, "report-2.txt")); errorValue != nil {
		t.Fatal("expected second distinct report kept as report-2.txt")
	}
}
