package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTestArgumentsUsesPromptAndDefaults(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	configuration, errorValue := parseTestArguments([]string{"저번 달 업무 보고서 워드 파일로 만들어줘"}, now)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.Prompt != "저번 달 업무 보고서 워드 파일로 만들어줘" {
		t.Fatalf("unexpected prompt: %q", configuration.Prompt)
	}
	if configuration.DownloadDirectoryPath != "/tmp/internkim-test-20260625T103000" {
		t.Fatalf("unexpected download directory: %s", configuration.DownloadDirectoryPath)
	}
	if configuration.OutputFilePath != "" {
		t.Fatalf("unexpected output file path: %s", configuration.OutputFilePath)
	}
	if !configuration.ShouldOpenFiles {
		t.Fatal("expected files to open by default")
	}
	if configuration.ShouldReuseFleet || configuration.ShouldKeepArtifacts {
		t.Fatalf("unexpected reuse/keep defaults: %+v", configuration)
	}
}

func TestParseTestArgumentsAcceptsFlagsAfterPrompt(t *testing.T) {
	configuration, errorValue := parseTestArguments([]string{
		"슬라이드 만들어줘",
		"--reuse",
		"--keep",
		"--no-open",
		"-o",
		"/tmp/custom.docx",
		"--timeout",
		"120",
	}, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.Prompt != "슬라이드 만들어줘" {
		t.Fatalf("unexpected prompt: %q", configuration.Prompt)
	}
	if !configuration.ShouldReuseFleet || !configuration.ShouldKeepArtifacts || configuration.ShouldOpenFiles {
		t.Fatalf("unexpected boolean flags: %+v", configuration)
	}
	if configuration.OutputFilePath != "/tmp/custom.docx" || configuration.TimeoutSeconds != 120 {
		t.Fatalf("unexpected value flags: %+v", configuration)
	}
}

func TestParseTestArgumentsAcceptsRunIDForDisposableFleet(t *testing.T) {
	configuration, errorValue := parseTestArguments([]string{
		"슬라이드 만들어줘",
		"--run-id",
		"quality-check",
	}, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.RunID != "quality-check" {
		t.Fatalf("unexpected run ID: %+v", configuration)
	}
}

func TestParseTestArgumentsRequiresPrompt(t *testing.T) {
	_, errorValue := parseTestArguments([]string{"--reuse"}, time.Now())
	if errorValue == nil || !strings.Contains(errorValue.Error(), "usage: internkim test") {
		t.Fatalf("expected usage error, got %v", errorValue)
	}
}

func TestParseTestArgumentsRejectsRunIDWithReuse(t *testing.T) {
	_, errorValue := parseTestArguments([]string{"프롬프트", "--reuse", "--run-id", "run-a"}, time.Now())
	if errorValue == nil || !strings.Contains(errorValue.Error(), "--run-id requires") {
		t.Fatalf("expected run-id reuse error, got %v", errorValue)
	}
}

func TestWriteTestDownloadedMattermostFilesWritesSingleAttachmentToOutputPath(t *testing.T) {
	outputFilePath := filepath.Join(t.TempDir(), "nested", "report.docx")
	output := `{"downloadedFiles":[{"fileID":"file-1","filename":"ignored.docx","contentBase64":"ZG9jeA=="}]}`
	downloadedFilePaths, errorValue := writeTestDownloadedMattermostFiles(output, outputFilePath, filepath.Join(t.TempDir(), "downloads"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(downloadedFilePaths) != 1 || downloadedFilePaths[0] != outputFilePath {
		t.Fatalf("unexpected downloaded file paths: %v", downloadedFilePaths)
	}
	content, errorValue := os.ReadFile(outputFilePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(content) != "docx" {
		t.Fatalf("unexpected output file content: %s", string(content))
	}
}

func TestWriteTestDownloadedMattermostFilesRejectsOutputPathForMultipleAttachments(t *testing.T) {
	output := `{"downloadedFiles":[{"fileID":"file-1","filename":"one.txt","contentBase64":"MQ=="},{"fileID":"file-2","filename":"two.txt","contentBase64":"Mg=="}]}`
	_, errorValue := writeTestDownloadedMattermostFiles(output, filepath.Join(t.TempDir(), "result.txt"), filepath.Join(t.TempDir(), "downloads"))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "-o can only write one Mattermost attachment") {
		t.Fatalf("expected multiple attachment output path error, got %v", errorValue)
	}
}
