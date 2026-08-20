package blueclaw

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runUsersSyncRequestHelpers(t *testing.T, shellBody string, arguments ...string) (string, error) {
	t.Helper()
	if _, lookupError := exec.LookPath("curl"); lookupError != nil {
		t.Skip("curl is required to exercise the users sync request helpers")
	}
	command := exec.Command("sh", append([]string{"-c", internKimUsersSyncRequestHelpers() + shellBody, "users-sync-test"}, arguments...)...)
	output, runError := command.CombinedOutput()
	return string(output), runError
}

func TestUsersSyncRequestHelpersReportTheResponseBodyOfAFailedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		http.Error(responseWriter, "open /delivery/config/policy.json.tmp: read-only file system", http.StatusInternalServerError)
	}))
	defer server.Close()

	responsePath := filepath.Join(t.TempDir(), "response")
	output, runError := runUsersSyncRequestHelpers(t, "\nrequest_or_exit \"invite sample@example.com\" \"$1\" \"$2\"\n", responsePath, server.URL)

	if runError == nil {
		t.Fatalf("expected a failing request to end the script, got output %q", output)
	}
	for _, fragment := range []string{
		"invite sample@example.com answered 500",
		"open /delivery/config/policy.json.tmp: read-only file system",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected the failure report to include %q, got %q", fragment, output)
		}
	}
}

func TestUsersSyncRequestHelpersKeepASuccessfulResponseBodyOnDisk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"revision":"abc"}`))
	}))
	defer server.Close()

	responsePath := filepath.Join(t.TempDir(), "response")
	output, runError := runUsersSyncRequestHelpers(t, "\nrequest_or_exit \"fleet user list\" \"$1\" \"$2\"\n", responsePath, server.URL)

	if runError != nil {
		t.Fatalf("expected a successful request to continue, got %v with output %q", runError, output)
	}
	if output != "" {
		t.Fatalf("expected a successful request to stay quiet, got %q", output)
	}
	body, readError := os.ReadFile(responsePath)
	if readError != nil {
		t.Fatalf("expected the response body on disk: %v", readError)
	}
	if string(body) != `{"revision":"abc"}` {
		t.Fatalf("expected the response body to be readable by the rest of the script, got %q", string(body))
	}
}

func TestUsersSyncRequestHelpersNameAnUnreachableServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	unreachableURL := server.URL
	server.Close()

	responsePath := filepath.Join(t.TempDir(), "response")
	output, runError := runUsersSyncRequestHelpers(t, "\nrequest_or_exit \"blueclaw policy read\" \"$1\" \"$2\"\n", responsePath, unreachableURL)

	if runError == nil {
		t.Fatalf("expected an unreachable server to end the script, got output %q", output)
	}
	if !strings.Contains(output, "blueclaw policy read never reached the server") {
		t.Fatalf("expected the transport failure to name the request, got %q", output)
	}
}

func TestUsersSyncScriptNeverDiscardsAFailedResponseBody(t *testing.T) {
	script := InternKimUsersSyncScript()

	if strings.Contains(script, "curl -f") {
		t.Fatal("curl --fail turns the server's explanation into a bare exit code; route requests through request_or_exit instead")
	}
	for _, fragment := range []string{
		`request_or_exit "fleet user list" "$response_path"`,
		`request_or_exit "invite $email" "$invite_response_path"`,
		`send_request "remove $email" "$removal_response_path"`,
		`report_response_body "remove $email" "$removal_response_path"`,
		"read_policy_snapshot",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected users sync script to include %q", fragment)
		}
	}
}

func TestUsersSyncScriptIsValidPosixShell(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "internkim-users-sync")
	if writeError := os.WriteFile(scriptPath, []byte(InternKimUsersSyncScript()), 0o755); writeError != nil {
		t.Fatalf("write script: %v", writeError)
	}
	output, runError := exec.Command("sh", "-n", scriptPath).CombinedOutput()
	if runError != nil {
		t.Fatalf("users sync script does not parse: %v\n%s", runError, output)
	}
}
