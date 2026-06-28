package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/tenantruntime"
)

type recordingTenantContainerCommandExecutor struct {
	runs    []tenantruntime.ContainerCommandInvocation
	outputs []tenantruntime.ContainerCommandInvocation
}

func (executor *recordingTenantContainerCommandExecutor) LookPath(name string) (string, error) {
	if name == "" {
		return "", errors.New("executable name is required")
	}
	return name, nil
}

func (executor *recordingTenantContainerCommandExecutor) CombinedOutput(invocation tenantruntime.ContainerCommandInvocation) ([]byte, error) {
	executor.outputs = append(executor.outputs, copyTenantContainerCommandInvocation(invocation))
	joinedArguments := strings.Join(invocation.Arguments, " ")
	if strings.Contains(joinedArguments, "token generate") {
		return []byte(`{"token":"tenant-token"}`), nil
	}
	if strings.Contains(joinedArguments, "run --rm --entrypoint cat") {
		return []byte(`<svg></svg>`), nil
	}
	if strings.Contains(joinedArguments, "team search") {
		return []byte(`[{"name":"tenant01"}]`), nil
	}
	if strings.Contains(joinedArguments, "ps tenant_01 --format json") {
		return []byte(`[{"State":"running"}]`), nil
	}
	return []byte{}, nil
}

func (executor *recordingTenantContainerCommandExecutor) Run(invocation tenantruntime.ContainerCommandInvocation) error {
	executor.runs = append(executor.runs, copyTenantContainerCommandInvocation(invocation))
	return nil
}

func TestExecuteTenantContainerCommandDispatchesInfraUp(t *testing.T) {
	workDirectoryPath := prepareTenantContainerCLIWorkDirectory(t)
	executor := &recordingTenantContainerCommandExecutor{}
	var output bytes.Buffer

	exitCode, errorValue := executeTenantContainerCommand("infra-up", []string{"--workdir", workDirectoryPath, "--tenant-count", "3"}, executor, &output, io.Discard)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if len(executor.runs) != 2 {
		t.Fatalf("expected two runs, got %+v", executor.runs)
	}
	expectedArguments := []string{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "up", "-d", "--wait"}
	if strings.Join(executor.runs[0].Arguments, "\x00") != strings.Join(expectedArguments, "\x00") {
		t.Fatalf("unexpected infra-up arguments: %+v", executor.runs[0].Arguments)
	}
	statement := tenantContainerRunStatement(t, executor.runs[1])
	if !strings.Contains(statement, `REVOKE ALL ON DATABASE "mattermost" FROM PUBLIC;`) {
		t.Fatalf("expected Mattermost database lockdown, got %s", statement)
	}
	if !strings.Contains(output.String(), `"infraComposePath"`) {
		t.Fatalf("expected JSON status output, got %s", output.String())
	}
}

func TestExecuteTenantContainerCommandAddWritesPerTenantDatabaseDSN(t *testing.T) {
	workDirectoryPath := prepareTenantContainerCLIWorkDirectory(t)
	installTenantContainerCLIHTTPTransport(t)
	executor := &recordingTenantContainerCommandExecutor{}
	var output bytes.Buffer

	exitCode, errorValue := executeTenantContainerCommand("add", []string{"--workdir", workDirectoryPath, "--tenant", "tenant01"}, executor, &output, io.Discard)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	password := strings.TrimSpace(readTenantContainerCLIFile(t, filepath.Join(workDirectoryPath, "secrets", "tenant_01", "db-password")))
	runtimeDocument := readTenantContainerCLIFile(t, filepath.Join(workDirectoryPath, "config", "tenant_01", "runtime.json"))
	expectedDatabaseSource := "postgres://tenant_01:" + password + "@postgres:5432/tenant_01?sslmode=disable"
	if !strings.Contains(runtimeDocument, expectedDatabaseSource) {
		t.Fatalf("expected per-tenant database source %q in %s", expectedDatabaseSource, runtimeDocument)
	}
	if strings.Contains(runtimeDocument, "postgres://internkim:internkim") {
		t.Fatalf("shared database source remained in %s", runtimeDocument)
	}
}

func TestExecuteTenantContainerCommandDispatchesStatus(t *testing.T) {
	workDirectoryPath := prepareTenantContainerCLIWorkDirectory(t)
	executor := &recordingTenantContainerCommandExecutor{}
	var output bytes.Buffer

	exitCode, errorValue := executeTenantContainerCommand("status", []string{"--workdir", workDirectoryPath}, executor, &output, io.Discard)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(output.String(), `"tenants": []`) {
		t.Fatalf("expected empty tenant status, got %s", output.String())
	}
}

func prepareTenantContainerCLIWorkDirectory(t *testing.T) string {
	t.Helper()
	workDirectoryPath := t.TempDir()
	if errorValue := os.MkdirAll(filepath.Join(workDirectoryPath, "secrets"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(workDirectoryPath, "secrets", "openrouter-key"), []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(workDirectoryPath, "cf.env"), []byte("CF_API_TOKEN=cf-token\nCF_ACCOUNT_ID=account-1\nCF_ZONE_ID=zone-1\nCF_TUNNEL_ID=tunnel-1\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return workDirectoryPath
}

func readTenantContainerCLIFile(t *testing.T, path string) string {
	t.Helper()
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(document)
}

func tenantContainerRunStatement(t *testing.T, invocation tenantruntime.ContainerCommandInvocation) string {
	t.Helper()
	document, errorValue := io.ReadAll(invocation.Stdin)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(document)
}

func copyTenantContainerCommandInvocation(invocation tenantruntime.ContainerCommandInvocation) tenantruntime.ContainerCommandInvocation {
	copiedInvocation := invocation
	copiedInvocation.Arguments = append([]string{}, invocation.Arguments...)
	copiedInvocation.Environment = append([]string{}, invocation.Environment...)
	return copiedInvocation
}

func installTenantContainerCLIHTTPTransport(t *testing.T) {
	t.Helper()
	originalTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = tenantContainerCLIHTTPTestTransport(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.Path == "/api/v4/users/login":
			return newTenantContainerCLIHTTPResponse(t, http.StatusOK, map[string]string{"id": "admin-id"}, "mattermost-token"), nil
		case request.URL.Path == "/api/v4/users/username/internkim01":
			return newTenantContainerCLIHTTPResponse(t, http.StatusOK, map[string]string{"id": "agent-id"}, ""), nil
		case request.URL.Path == "/api/v4/users/username/admin01":
			return newTenantContainerCLIHTTPResponse(t, http.StatusOK, map[string]string{"id": "company-admin-id"}, ""), nil
		case request.URL.Path == "/api/v4/teams/name/tenant01":
			return newTenantContainerCLIHTTPResponse(t, http.StatusOK, map[string]string{"id": "team-id"}, ""), nil
		case request.URL.Path == "/api/v4/users/agent-id/patch":
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["nickname"] != "김인턴" || payload["first_name"] != "Intern" || payload["last_name"] != "Kim" {
				t.Fatalf("unexpected agent profile patch: %#v", payload)
			}
			return newTenantContainerCLIHTTPResponse(t, http.StatusOK, map[string]bool{"ok": true}, ""), nil
		case request.URL.Path == "/api/v4/users/agent-id/image":
			file, fileHeader, errorValue := request.FormFile("image")
			if errorValue != nil {
				t.Fatalf("expected profile image upload: %v", errorValue)
			}
			defer file.Close()
			document, errorValue := io.ReadAll(file)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if fileHeader.Filename != "internkim.png" || !bytes.HasPrefix(document, []byte("\x89PNG\r\n\x1a\n")) {
				t.Fatalf("unexpected profile image upload: %s %d bytes", fileHeader.Filename, len(document))
			}
			return newTenantContainerCLIHTTPResponse(t, http.StatusOK, map[string]bool{"ok": true}, ""), nil
		case request.URL.Path == "/api/v4/teams/team-id/members/company-admin-id/schemeRoles":
			return newTenantContainerCLIHTTPResponse(t, http.StatusOK, map[string]bool{"ok": true}, ""), nil
		case request.URL.Path == "/api/v4/users/company-admin-id/patch":
			return newTenantContainerCLIHTTPResponse(t, http.StatusOK, map[string]bool{"ok": true}, ""), nil
		case request.URL.Path == "/api/v4/channels/direct":
			return newTenantContainerCLIHTTPResponse(t, http.StatusCreated, map[string]string{"id": "dm-channel-id"}, ""), nil
		case strings.Contains(request.URL.Path, "/accounts/account-1/cfd_tunnel/tunnel-1/configurations") && request.Method == http.MethodGet:
			value := map[string]any{"success": true, "result": map[string]any{"config": map[string]any{"ingress": []map[string]string{{"hostname": "poc-0.intern.kim", "service": "http://mattermost:8065"}, {"service": "http_status:404"}}}}}
			return newTenantContainerCLIHTTPResponse(t, http.StatusOK, value, ""), nil
		case strings.Contains(request.URL.Path, "/accounts/account-1/cfd_tunnel/tunnel-1/configurations") && request.Method == http.MethodPut:
			return newTenantContainerCLIHTTPResponse(t, http.StatusOK, map[string]bool{"success": true}, ""), nil
		case strings.Contains(request.URL.Path, "/zones/zone-1/dns_records") && request.Method == http.MethodGet:
			return newTenantContainerCLIHTTPResponse(t, http.StatusOK, map[string]any{"success": true, "result": []any{}}, ""), nil
		case strings.Contains(request.URL.Path, "/zones/zone-1/dns_records") && request.Method == http.MethodPost:
			return newTenantContainerCLIHTTPResponse(t, http.StatusOK, map[string]bool{"success": true}, ""), nil
		default:
			t.Fatalf("unexpected HTTP request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultClient.Transport = originalTransport
	})
}

type tenantContainerCLIHTTPTestTransport func(request *http.Request) (*http.Response, error)

func (transport tenantContainerCLIHTTPTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func newTenantContainerCLIHTTPResponse(t *testing.T, statusCode int, value any, token string) *http.Response {
	t.Helper()
	document, errorValue := json.Marshal(value)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	header := http.Header{"Content-Type": []string{"application/json"}}
	if token != "" {
		header.Set("Token", token)
	}
	return &http.Response{StatusCode: statusCode, Header: header, Body: io.NopCloser(strings.NewReader(string(document)))}
}
