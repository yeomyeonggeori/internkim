package cli

import (
	"encoding/base64"
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
		Files:        []downloadedMattermostFile{file},
		Steps: []mattermostScenarioStepResult{{
			Attachments: []downloadedMattermostFile{file},
			TaskEvents:  []mattermostScenarioTaskEvent{{TaskEventID: "event-1", Name: "task.completed"}},
		}},
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
}
