package companion

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestExecutorMountCreateListAndWrite(t *testing.T) {
	rootPath := t.TempDir()
	executor := Executor{MountStore: NewMountStore("")}

	createInput, errorValue := json.Marshal(map[string]string{"path": rootPath, "displayName": "Work"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	createResponse, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "filesystem_mount_create",
		Input:    createInput,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var mount MountSnapshot
	if errorValue := json.Unmarshal(createResponse.Result, &mount); errorValue != nil {
		t.Fatal(errorValue)
	}

	writeInput, errorValue := json.Marshal(map[string]string{"mountID": mount.MountID, "path": "report.txt", "content": "hello"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "filesystem_mount_write",
		Input:    writeInput,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if document, errorValue := os.ReadFile(filepath.Join(rootPath, "report.txt")); errorValue != nil || string(document) != "hello" {
		t.Fatalf("expected mounted file write, got %q %v", string(document), errorValue)
	}

	listResponse, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{ToolName: "filesystem_mount_list"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(listResponse.Result), mount.GuestPath) {
		t.Fatalf("expected list to include guest path, got %s", listResponse.Result)
	}
}
