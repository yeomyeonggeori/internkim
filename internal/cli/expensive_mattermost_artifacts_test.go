package cli

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteExpensiveMattermostEvidencePersistsFilesWithoutEmbeddingContent(t *testing.T) {
	directoryPath := t.TempDir()
	file := downloadedMattermostFile{
		FileID:        "file-1",
		Filename:      "분기 결산.docx",
		ContentType:   "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("document-content")),
	}
	result := mattermostScenarioResult{
		ScenarioName: "document-lifecycle",
		TurnCount:    1,
		Steps: []mattermostScenarioStepResult{{
			TaskRunID:      "task-1",
			TaskStatus:     "completed",
			LLMCallCount:   2,
			AgentStepCount: 1,
			ToolCallCount:  1,
			ProcessingMS:   1234,
			Attachments:    []downloadedMattermostFile{file},
			TaskEvents:     []mattermostScenarioTaskEvent{{TaskEventID: "event-1", Name: "task.completed"}},
		}},
	}
	uiDirectoryPath := filepath.Join(directoryPath, "ui", "step-01")
	if errorValue := os.MkdirAll(uiDirectoryPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(uiDirectoryPath, "mattermost-dm.png"), []byte("screenshot"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := writeExpensiveMattermostEvidence(directoryPath, result); errorValue != nil {
		t.Fatal(errorValue)
	}
	documentPath := filepath.Join(directoryPath, "files", "step-01", "분기 결산.docx")
	document, errorValue := os.ReadFile(documentPath)
	if errorValue != nil || string(document) != "document-content" {
		t.Fatalf("document=%q error=%v", document, errorValue)
	}
	resultDocument, errorValue := os.ReadFile(filepath.Join(directoryPath, "result.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(resultDocument), file.ContentBase64) || strings.Contains(string(resultDocument), "document-content") {
		t.Fatal("result JSON contains embedded attachment content")
	}
	eventsDocument, errorValue := os.ReadFile(filepath.Join(directoryPath, "events", "step-01.json"))
	if errorValue != nil || !strings.Contains(string(eventsDocument), "event-1") {
		t.Fatalf("events=%s error=%v", eventsDocument, errorValue)
	}
	manifestDocument, errorValue := os.ReadFile(filepath.Join(directoryPath, "manifest.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var manifest expensiveMattermostEvidenceManifest
	if errorValue := json.Unmarshal(manifestDocument, &manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	if manifest.SchemaVersion != 1 || manifest.ResultPath != "result.json" || len(manifest.Steps) != 1 {
		t.Fatalf("unexpected manifest=%+v", manifest)
	}
	step := manifest.Steps[0]
	if step.TaskRunID != "task-1" || step.EventPath != "events/step-01.json" || step.LLMCallCount != 2 || step.ProcessingMS != 1234 {
		t.Fatalf("unexpected manifest step=%+v", step)
	}
	if len(step.AttachmentPaths) != 1 || step.AttachmentPaths[0] != "files/step-01/분기 결산.docx" {
		t.Fatalf("unexpected attachment paths=%v", step.AttachmentPaths)
	}
	if len(step.UIArtifactPaths) != 1 || step.UIArtifactPaths[0] != "ui/step-01/mattermost-dm.png" {
		t.Fatalf("unexpected UI paths=%v", step.UIArtifactPaths)
	}
}

func TestMarshalEnvironmentStringArrayNormalizesNil(t *testing.T) {
	if document := marshalEnvironmentStringArray(nil); document != "[]" {
		t.Fatalf("expected empty JSON array, got %s", document)
	}
}

func TestMarshalEnvironmentStringArrayPreservesValues(t *testing.T) {
	if document := marshalEnvironmentStringArray([]string{"보고서.pdf", "확인"}); document != `["보고서.pdf","확인"]` {
		t.Fatalf("unexpected JSON array %s", document)
	}
}

func TestLatestMattermostScenarioPostReturnsLastReply(t *testing.T) {
	posts := []mattermostScenarioPost{{ID: "request"}, {ID: "reply", Message: "확인할까요?"}}

	post, found := latestMattermostScenarioPost(posts)

	if !found || post.ID != "reply" || post.Message != "확인할까요?" {
		t.Fatalf("post=%+v found=%t", post, found)
	}
}

func TestLatestMattermostScenarioPostRejectsEmptyConversation(t *testing.T) {
	if _, found := latestMattermostScenarioPost(nil); found {
		t.Fatal("expected no latest post")
	}
}
