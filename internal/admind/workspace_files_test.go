package admind

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const workspaceFilesTestPolicy = `{
	"people":[
		{"personID":"person-me","emails":["me@example.com"],"circles":["engineering","staff"]},
		{"personID":"person-other","emails":["other@example.com"],"circles":["design"]}
	],
	"circles":[
		{"circleID":"engineering","displayName":"Engineering"},
		{"circleID":"design","displayName":"Design"},
		{"circleID":"staff","displayName":"Staff"}
	]
}`

func newWorkspaceFilesTestService(t *testing.T) (*Service, string) {
	t.Helper()
	workspaceDirectory := t.TempDir()
	seedWorkspaceFile(t, workspaceDirectory, "private/people/person-me/note.txt", "mine")
	seedWorkspaceFile(t, workspaceDirectory, "private/people/person-me/.blueclaw/internal.txt", "secret-internal")
	seedWorkspaceFile(t, workspaceDirectory, "private/people/person-other/private.txt", "not-yours")
	seedWorkspaceFile(t, workspaceDirectory, "circles/engineering/spec.md", "engineering-spec")
	seedWorkspaceFile(t, workspaceDirectory, "circles/design/confidential.txt", "design-only")
	seedWorkspaceFile(t, workspaceDirectory, "shared/public/readme.txt", "everyone")
	seedWorkspaceFile(t, workspaceDirectory, "shared/cache/dependencies/cached.txt", "package-cache")
	service := NewService(Configuration{
		BlueclawBaseURL:       "http://blueclaw.local",
		BlueclawWorkspacePath: workspaceDirectory,
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/admin/api/policy" {
			return jsonResponse(http.StatusOK, workspaceFilesTestPolicy, nil), nil
		}
		if request.URL.Path == "/admin/api/workspace/list" {
			return workspaceFilesMockList(workspaceDirectory, request.URL.Query().Get("path")), nil
		}
		if request.URL.Path == "/admin/api/workspace/download" {
			return workspaceFilesMockDownload(workspaceDirectory, request.URL.Query().Get("path")), nil
		}
		return jsonResponse(http.StatusNotFound, `{}`, nil), nil
	})}
	return service, workspaceDirectory
}

func workspaceFilesMockHostPath(workspaceDirectory string, agentPath string) string {
	return filepath.Join(workspaceDirectory, strings.TrimPrefix(agentPath, "/workspace"))
}

func workspaceFilesMockList(workspaceDirectory string, agentPath string) *http.Response {
	directoryEntries, errorValue := os.ReadDir(workspaceFilesMockHostPath(workspaceDirectory, agentPath))
	if errorValue != nil {
		return jsonResponse(http.StatusOK, `{"entries":[]}`, nil)
	}
	entries := []map[string]any{}
	for _, directoryEntry := range directoryEntries {
		if directoryEntry.Name() == ".blueclaw" {
			continue
		}
		entries = append(entries, map[string]any{"name": directoryEntry.Name(), "isDirectory": directoryEntry.IsDir()})
	}
	document, _ := json.Marshal(map[string]any{"entries": entries})
	return jsonResponse(http.StatusOK, string(document), nil)
}

func workspaceFilesMockDownload(workspaceDirectory string, agentPath string) *http.Response {
	content, errorValue := os.ReadFile(workspaceFilesMockHostPath(workspaceDirectory, agentPath))
	if errorValue != nil {
		return jsonResponse(http.StatusNotFound, ``, nil)
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(content))), Header: http.Header{}}
}

func seedWorkspaceFile(t *testing.T, workspaceDirectory string, relativePath string, content string) {
	t.Helper()
	fullPath := filepath.Join(workspaceDirectory, filepath.FromSlash(relativePath))
	if errorValue := os.MkdirAll(filepath.Dir(fullPath), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(fullPath, []byte(content), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func workspaceFilesRequest(t *testing.T, service *Service, method string, target string, email string, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var requestBody *bytes.Buffer = body
	if requestBody == nil {
		requestBody = &bytes.Buffer{}
	}
	request := httptest.NewRequest(method, target, requestBody)
	if email != "" {
		request.Header.Set("X-Forwarded-Email", email)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	service.handleFiles(recorder, request)
	return recorder
}

func TestWorkspaceFilesRootsListsOwnMemberCirclesAndShared(t *testing.T) {
	service, _ := newWorkspaceFilesTestService(t)
	recorder := workspaceFilesRequest(t, service, http.MethodGet, "/files/api/roots", "me@example.com", nil, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Roots []workspaceRoot `json:"roots"`
	}
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	agentPaths := map[string]bool{}
	for _, root := range response.Roots {
		agentPaths[root.AgentPath] = true
	}
	for _, expected := range []string{"/workspace/private/people/person-me", "/workspace/circles/engineering", "/workspace/circles/staff", "/workspace/shared/public"} {
		if !agentPaths[expected] {
			t.Fatalf("missing root %q in %+v", expected, response.Roots)
		}
	}
	if agentPaths["/workspace/circles/design"] {
		t.Fatalf("non-member circle leaked into roots: %+v", response.Roots)
	}
}

func TestWorkspaceFilesListAllowsOwnCircleAndShared(t *testing.T) {
	service, _ := newWorkspaceFilesTestService(t)
	cases := []struct {
		path     string
		expected string
	}{
		{"/workspace/private/people/person-me", "note.txt"},
		{"/workspace/circles/engineering", "spec.md"},
		{"/workspace/shared/public", "readme.txt"},
	}
	for _, testCase := range cases {
		recorder := workspaceFilesRequest(t, service, http.MethodGet, "/files/api/list?path="+testCase.path, "me@example.com", nil, "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("list %q status = %d body = %s", testCase.path, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), testCase.expected) {
			t.Fatalf("list %q missing %q: %s", testCase.path, testCase.expected, recorder.Body.String())
		}
	}
}

func TestWorkspaceFilesListExcludesBlueclawInternals(t *testing.T) {
	service, _ := newWorkspaceFilesTestService(t)
	recorder := workspaceFilesRequest(t, service, http.MethodGet, "/files/api/list?path=/workspace/private/people/person-me", "me@example.com", nil, "")
	if strings.Contains(recorder.Body.String(), ".blueclaw") {
		t.Fatalf("listing leaked .blueclaw entry: %s", recorder.Body.String())
	}
}

func TestWorkspaceFilesDeniesForbiddenPaths(t *testing.T) {
	service, _ := newWorkspaceFilesTestService(t)
	forbidden := []string{
		"/workspace/circles/design",
		"/workspace/private/people/person-other",
		"/workspace/private/people/person-me/../person-other",
		"/workspace/shared",
		"/workspace/shared/cache/dependencies",
	}
	for _, path := range forbidden {
		recorder := workspaceFilesRequest(t, service, http.MethodGet, "/files/api/list?path="+path, "me@example.com", nil, "")
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for %q, got %d body = %s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestWorkspaceFilesDeniesBlueclawInternalPath(t *testing.T) {
	service, _ := newWorkspaceFilesTestService(t)
	recorder := workspaceFilesRequest(t, service, http.MethodGet, "/files/api/list?path=/workspace/private/people/person-me/.blueclaw", "me@example.com", nil, "")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for .blueclaw path, got %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestWorkspaceFilesDownloadAndDenial(t *testing.T) {
	service, _ := newWorkspaceFilesTestService(t)
	recorder := workspaceFilesRequest(t, service, http.MethodGet, "/files/api/download?path=/workspace/private/people/person-me/note.txt", "me@example.com", nil, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("download status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != "mine" {
		t.Fatalf("download body = %q", recorder.Body.String())
	}
	if !strings.Contains(recorder.Header().Get("Content-Disposition"), "note.txt") {
		t.Fatalf("missing attachment disposition: %q", recorder.Header().Get("Content-Disposition"))
	}
	deniedRecorder := workspaceFilesRequest(t, service, http.MethodGet, "/files/api/download?path=/workspace/circles/design/confidential.txt", "me@example.com", nil, "")
	if deniedRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 download, got %d", deniedRecorder.Code)
	}
}

func TestWorkspaceFilesUploadWritesIntoAllowedDirectoryAndDeniesOthers(t *testing.T) {
	service, workspaceDirectory := newWorkspaceFilesTestService(t)
	body, contentType := buildWorkspaceUploadBody(t, "upload.txt", "uploaded-content")
	recorder := workspaceFilesRequest(t, service, http.MethodPost, "/files/api/upload?path=/workspace/circles/engineering", "me@example.com", body, contentType)
	if recorder.Code != http.StatusOK {
		t.Fatalf("upload status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	written, errorValue := os.ReadFile(filepath.Join(workspaceDirectory, "circles", "engineering", "upload.txt"))
	if errorValue != nil {
		t.Fatalf("uploaded file not written: %v", errorValue)
	}
	if string(written) != "uploaded-content" {
		t.Fatalf("uploaded content = %q", string(written))
	}
	deniedBody, deniedContentType := buildWorkspaceUploadBody(t, "evil.txt", "x")
	deniedRecorder := workspaceFilesRequest(t, service, http.MethodPost, "/files/api/upload?path=/workspace/circles/design", "me@example.com", deniedBody, deniedContentType)
	if deniedRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 upload, got %d body = %s", deniedRecorder.Code, deniedRecorder.Body.String())
	}
}

func TestWorkspaceFilesRejectsUnknownPerson(t *testing.T) {
	service, _ := newWorkspaceFilesTestService(t)
	recorder := workspaceFilesRequest(t, service, http.MethodGet, "/files/api/roots", "stranger@example.com", nil, "")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unknown person, got %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func buildWorkspaceUploadBody(t *testing.T, fileName string, content string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, errorValue := writer.CreateFormFile("files", fileName)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := part.Write([]byte(content)); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := writer.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	return body, writer.FormDataContentType()
}

func TestCleanWorkspaceHostPathRejectsSymlinkEscape(t *testing.T) {
	workspaceDirectory := t.TempDir()
	outsideDirectory := t.TempDir()
	if errorValue := os.WriteFile(filepath.Join(outsideDirectory, "secret.txt"), []byte("x"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.Symlink(outsideDirectory, filepath.Join(workspaceDirectory, "escape")); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := cleanWorkspaceHostPath(workspaceDirectory, filepath.Join(workspaceDirectory, "escape", "secret.txt")); errorValue == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestCleanWorkspaceAgentPathValidation(t *testing.T) {
	valid := []string{"/workspace/shared", "/workspace/circles/engineering/spec.md"}
	for _, path := range valid {
		if _, errorValue := cleanWorkspaceAgentPath(path); errorValue != nil {
			t.Fatalf("expected %q valid: %v", path, errorValue)
		}
	}
	invalid := []string{"", "workspace/shared", "/etc/passwd", "/workspace/.blueclaw/config"}
	for _, path := range invalid {
		if _, errorValue := cleanWorkspaceAgentPath(path); errorValue == nil {
			t.Fatalf("expected %q invalid", path)
		}
	}
}

func TestWorkspaceUploadFileNameRejectsTraversal(t *testing.T) {
	cases := map[string]bool{
		"report.pdf":        true,
		"nested/report.pdf": true,
		"../escape.txt":     true,
		"..":                false,
		"":                  false,
		".blueclaw":         false,
	}
	for raw, expectedAccepted := range cases {
		fileName, accepted := sanitizeWorkspaceUploadFileName(raw)
		if accepted != expectedAccepted {
			t.Fatalf("name %q accepted = %v, want %v", raw, accepted, expectedAccepted)
		}
		if accepted && strings.ContainsRune(fileName, '/') {
			t.Fatalf("accepted name %q still contains a separator: %q", raw, fileName)
		}
	}
}
