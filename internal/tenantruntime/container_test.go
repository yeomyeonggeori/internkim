package tenantruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/botassets"
)

type recordingContainerCommandExecutor struct {
	runs    []ContainerCommandInvocation
	outputs []ContainerCommandInvocation
}

type recordingTenantPostgresDatabase struct {
	statements []string
}

func (executor *recordingContainerCommandExecutor) LookPath(name string) (string, error) {
	if name == "" {
		return "", errors.New("executable name is required")
	}
	return name, nil
}

func (executor *recordingContainerCommandExecutor) CombinedOutput(invocation ContainerCommandInvocation) ([]byte, error) {
	executor.outputs = append(executor.outputs, copyContainerCommandInvocation(invocation))
	joinedArguments := strings.Join(invocation.Arguments, " ")
	if strings.Contains(joinedArguments, "token generate") {
		return []byte(`{"token":"tenant-token"}`), nil
	}
	if strings.Contains(joinedArguments, "team search") {
		return []byte(`[{"name":"tenant01"}]`), nil
	}
	if strings.Contains(joinedArguments, "ps tenant_01 --format json") {
		return []byte(`[{"State":"running"}]`), nil
	}
	return []byte{}, nil
}

func (executor *recordingContainerCommandExecutor) Run(invocation ContainerCommandInvocation) error {
	executor.runs = append(executor.runs, copyContainerCommandInvocation(invocation))
	return nil
}

func (database *recordingTenantPostgresDatabase) ExecutePostgres(ctx context.Context, statement string) error {
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}
	database.statements = append(database.statements, statement)
	return nil
}

func TestContainerTenantForIndexZeroPadsTenantNames(t *testing.T) {
	tenant, errorValue := ContainerTenantForIndex(1)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if tenant.TenantID != "tenant01" || tenant.RuntimeID != "tenant_01" || tenant.DatabaseName != "tenant_01" || tenant.AgentUsername != "internkim01" {
		t.Fatalf("unexpected tenant: %+v", tenant)
	}
	parsedTenant, errorValue := ParseContainerTenant("tenant_10")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if parsedTenant.TenantID != "tenant10" || parsedTenant.RuntimeID != "tenant_10" {
		t.Fatalf("unexpected parsed tenant: %+v", parsedTenant)
	}
	if _, errorValue := ParseContainerTenant("tenant1"); errorValue == nil {
		t.Fatal("expected non-zero-padded tenant name to fail")
	}
}

func TestRenderContainerTenantComposeUsesPocServiceShape(t *testing.T) {
	firstTenant, errorValue := ContainerTenantForIndex(1)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secondTenant, errorValue := ContainerTenantForIndex(10)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	workDirectoryPath := filepath.Join(t.TempDir(), "container-poc")
	document := RenderContainerTenantCompose([]ContainerTenant{firstTenant, secondTenant}, "custom-tenant:latest", filepath.Join(workDirectoryPath, "secrets", "openrouter-key"), workDirectoryPath)
	for _, fragment := range []string{
		"name: internkim-poc-tenants",
		"  internkim-poc:",
		"  tenant_01:",
		"    image: custom-tenant:latest",
		"        aliases: [tenant_01]",
		"      ENABLE_ADMIND: \"1\"",
		"      - ./config/tenant_01:/etc/blueclaw:rw",
		"      - ./secrets/openrouter-key:/secrets/openrouter-key:ro",
		"      - ./secrets/tenant_10/mattermost-bot-token:/secrets/mattermost-bot-token:ro",
		"      - ./secrets/tenant_10/mattermost-bot-token:/root/.internkim/secrets/mattermost-bot-token:ro",
		"      - ./secrets/tenant_10/mm-admin-pass:/root/.internkim/secrets/mm-admin-pass:ro",
		"    restart: on-failure",
	} {
		if !strings.Contains(document, fragment) {
			t.Fatalf("compose document missing %q:\n%s", fragment, document)
		}
	}
}

func TestParseMattermostTokenReadsJSONToken(t *testing.T) {
	token, errorValue := ParseMattermostToken([]byte(`{"token":"abc123"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if token != "abc123" {
		t.Fatalf("unexpected token: %s", token)
	}
	if _, errorValue := ParseMattermostToken([]byte(`{"id":"missing"}`)); errorValue == nil {
		t.Fatal("expected missing token to fail")
	}
	arrayToken, errorValue := ParseMattermostToken([]byte(`[{"id":"x","token":"arr789","is_active":true}]`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if arrayToken != "arr789" {
		t.Fatalf("unexpected array token: %s", arrayToken)
	}
}

func TestCreateTenantRoleBuildsLockedTenantDatabaseSQL(t *testing.T) {
	database := &recordingTenantPostgresDatabase{}

	errorValue := createTenantRole(context.Background(), database, "tenant_01", "tenant_database_password_0123456789")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(database.statements) != 1 {
		t.Fatalf("expected one statement, got %+v", database.statements)
	}
	statement := database.statements[0]
	for _, fragment := range []string{
		`CREATE ROLE "tenant_01" WITH LOGIN PASSWORD 'tenant_database_password_0123456789';`,
		`CREATE DATABASE "tenant_01" OWNER "tenant_01"`,
		`ALTER DATABASE "tenant_01" OWNER TO "tenant_01";`,
		`REVOKE CONNECT ON DATABASE "tenant_01" FROM PUBLIC;`,
		`GRANT CONNECT ON DATABASE "tenant_01" TO "tenant_01";`,
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("tenant role SQL missing %q:\n%s", fragment, statement)
		}
	}
}

func TestCreateTenantRoleRejectsUnsafeRuntimeID(t *testing.T) {
	database := &recordingTenantPostgresDatabase{}

	errorValue := createTenantRole(context.Background(), database, `tenant_01"; DROP ROLE internkim; --`, "tenant_database_password_0123456789")
	if errorValue == nil {
		t.Fatal("expected unsafe runtime id to fail")
	}
	if len(database.statements) != 0 {
		t.Fatalf("expected no SQL for unsafe runtime id, got %+v", database.statements)
	}
}

func TestDropTenantRoleBuildsCleanupSQL(t *testing.T) {
	database := &recordingTenantPostgresDatabase{}

	errorValue := dropTenantRole(context.Background(), database, "tenant_01")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	statement := database.statements[0]
	for _, fragment := range []string{
		`DROP DATABASE IF EXISTS "tenant_01";`,
		`DROP ROLE IF EXISTS "tenant_01";`,
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("tenant drop SQL missing %q:\n%s", fragment, statement)
		}
	}
}

func TestContainerAddOrdersDockerAndMattermostSteps(t *testing.T) {
	workDirectoryPath := prepareContainerRuntimeWorkDirectory(t)
	installContainerRuntimeHTTPTransport(t)
	executor := &recordingContainerCommandExecutor{}
	runtime := ContainerRuntime{CommandExecutor: executor, Output: io.Discard, ErrorOutput: io.Discard}

	status, errorValue := runtime.AddTenant(ContainerTenantAddOptions{WorkDirectoryPath: workDirectoryPath, TenantID: "tenant01"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if status.ContainerState != "running" || !status.MattermostTeamPresent {
		t.Fatalf("unexpected status: %+v", status)
	}
	expectedRunArguments := [][]string{
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "postgres", "psql", "-v", "ON_ERROR_STOP=1", "-U", "internkim", "-d", "postgres"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "user", "create", "--email", "admin@intern.kim", "--username", "admin", "--password", readContainerTestFileTrimmed(t, containerMattermostAdminPasswordPath(workDirectoryPath)), "--system-admin"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "user", "change-password", "admin", "--password", readContainerTestFileTrimmed(t, containerMattermostAdminPasswordPath(workDirectoryPath))},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "config", "set", "TeamSettings.TeammateNameDisplay", "nickname_full_name"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "config", "set", "LocalizationSettings.DefaultClientLocale", "ko"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "config", "set", "LocalizationSettings.DefaultServerLocale", "ko"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "team", "create", "--name", "tenant01", "--display-name", "Tenant 01"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "team", "users", "add", "tenant01", "admin"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "bot", "create", "internkim01", "--display-name", "김인턴"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "user", "convert", "internkim01", "--bot"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "team", "users", "add", "tenant01", "internkim01"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "user", "create", "--email", "admin01@intern.kim", "--username", "admin01", "--password", readContainerTestFileTrimmed(t, tenantCompanyAdminPasswordPath(workDirectoryPath, mustContainerTenant(t, 1)))},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "user", "change-password", "admin01", "--password", readContainerTestFileTrimmed(t, tenantCompanyAdminPasswordPath(workDirectoryPath, mustContainerTenant(t, 1)))},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "team", "users", "add", "tenant01", "admin01"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "tenants.generated.yml"), "up", "-d", "--force-recreate", "tenant_01"},
	}
	assertContainerRunArguments(t, executor.runs, expectedRunArguments)
	assertFileContains(t, filepath.Join(workDirectoryPath, "config", "tenant_01", "runtime.json"), `"mode": "native"`)
	databasePassword := strings.TrimSpace(readContainerTestFile(t, tenantDatabasePasswordPath(workDirectoryPath, mustContainerTenant(t, 1))))
	assertFileContains(t, filepath.Join(workDirectoryPath, "config", "tenant_01", "runtime.json"), "postgres://tenant_01:"+databasePassword+"@postgres:5432/tenant_01?sslmode=disable")
	assertFileDoesNotContain(t, filepath.Join(workDirectoryPath, "config", "tenant_01", "runtime.json"), "postgres://internkim:internkim")
	roleStatement := containerRunStatement(t, executor.runs[0])
	assertStringContains(t, roleStatement, `CREATE ROLE "tenant_01" WITH LOGIN PASSWORD`)
	assertStringContains(t, roleStatement, `REVOKE CONNECT ON DATABASE "tenant_01" FROM PUBLIC;`)
	assertStringContains(t, roleStatement, `GRANT CONNECT ON DATABASE "tenant_01" TO "tenant_01";`)
	assertFileContains(t, filepath.Join(workDirectoryPath, "secrets", "tenant_01", "mattermost-bot-token"), "tenant-token")
	assertFileContains(t, filepath.Join(workDirectoryPath, "secrets", "tenant_01", "admin-email"), "admin01@intern.kim")
	assertFileContains(t, filepath.Join(workDirectoryPath, "secrets", "tenant_01", "device-url"), "https://poc-0.intern.kim")
}

func TestContainerAddReusesStoredMattermostPasswordsWithoutRotation(t *testing.T) {
	workDirectoryPath := prepareContainerRuntimeWorkDirectory(t)
	tenant := mustContainerTenant(t, 1)
	if errorValue := os.MkdirAll(containerTenantSecretPath(workDirectoryPath, tenant), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(containerMattermostAdminPasswordPath(workDirectoryPath), []byte("operator-password\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(tenantCompanyAdminPasswordPath(workDirectoryPath, tenant), []byte("company-password\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	installContainerRuntimeHTTPTransport(t)
	executor := &recordingContainerCommandExecutor{}
	runtime := ContainerRuntime{CommandExecutor: executor, Output: io.Discard, ErrorOutput: io.Discard}

	status, errorValue := runtime.AddTenant(ContainerTenantAddOptions{WorkDirectoryPath: workDirectoryPath, TenantID: "tenant01"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if status.CompanyAdminPassword != "company-password" {
		t.Fatalf("company admin password = %q", status.CompanyAdminPassword)
	}
	for _, invocation := range executor.runs {
		if strings.Contains(strings.Join(invocation.Arguments, " "), "change-password") {
			t.Fatalf("stored password add rotated an account: %+v", invocation.Arguments)
		}
	}
	if readContainerTestFileTrimmed(t, containerMattermostAdminPasswordPath(workDirectoryPath)) != "operator-password" {
		t.Fatal("operator admin password changed")
	}
	if readContainerTestFileTrimmed(t, tenantCompanyAdminPasswordPath(workDirectoryPath, tenant)) != "company-password" {
		t.Fatal("company admin password changed")
	}
}

func TestContainerRemoveOrdersCleanupSteps(t *testing.T) {
	workDirectoryPath := prepareContainerRuntimeWorkDirectory(t)
	mustCreateContainerTenantDirectories(t, workDirectoryPath, "tenant_01")
	executor := &recordingContainerCommandExecutor{}
	runtime := ContainerRuntime{CommandExecutor: executor, Output: io.Discard, ErrorOutput: io.Discard}

	status, errorValue := runtime.RemoveTenant(ContainerTenantRemoveOptions{WorkDirectoryPath: workDirectoryPath, TenantID: "tenant01", PurgeData: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if !status.PurgedDatabase || !status.RemovedConfig || !status.RemovedSecrets || !status.UpdatedCompose {
		t.Fatalf("unexpected removal status: %+v", status)
	}
	expectedRunArguments := [][]string{
		{"compose", "-f", filepath.Join(workDirectoryPath, "tenants.generated.yml"), "rm", "--stop", "--force", "tenant_01"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "team", "delete", "tenant01", "--confirm"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "mattermost", "mmctl", "--local", "user", "delete", "internkim01", "--confirm"},
		{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "exec", "-T", "postgres", "psql", "-v", "ON_ERROR_STOP=1", "-U", "internkim", "-d", "postgres"},
	}
	assertContainerRunArguments(t, executor.runs, expectedRunArguments)
	dropStatement := containerRunStatement(t, executor.runs[3])
	assertStringContains(t, dropStatement, `DROP DATABASE IF EXISTS "tenant_01";`)
	assertStringContains(t, dropStatement, `DROP ROLE IF EXISTS "tenant_01";`)
	if fileExists(filepath.Join(workDirectoryPath, "secrets", "tenant_01", "db-password")) {
		t.Fatal("expected tenant database password to be removed")
	}
}

func TestContainerResetPurgesTenantsThenInfra(t *testing.T) {
	workDirectoryPath := prepareContainerRuntimeWorkDirectory(t)
	mustCreateContainerTenantDirectories(t, workDirectoryPath, "tenant_01")
	mustCreateContainerTenantDirectories(t, workDirectoryPath, "tenant_02")
	superAdminPasswordPath := containerMattermostAdminPasswordPath(workDirectoryPath)
	if errorValue := os.WriteFile(superAdminPasswordPath, []byte("operator-password\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	installContainerRuntimeHTTPTransport(t)
	executor := &recordingContainerCommandExecutor{}
	runtime := ContainerRuntime{CommandExecutor: executor, Output: io.Discard, ErrorOutput: io.Discard}

	status, errorValue := runtime.Reset(ContainerResetOptions{WorkDirectoryPath: workDirectoryPath, Purge: true, ConfirmSuperAdmin: true, SuperAdminPasswordPath: superAdminPasswordPath})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(status.RemovedTenants) != 2 || !status.InfraDown || !status.SuperAdminVerified {
		t.Fatalf("unexpected reset status: %+v", status)
	}
	lastRun := executor.runs[len(executor.runs)-1]
	expectedArguments := []string{"compose", "-f", filepath.Join(workDirectoryPath, "infra", "docker-compose.yml"), "down", "--volumes"}
	if !reflect.DeepEqual(lastRun.Arguments, expectedArguments) {
		t.Fatalf("unexpected final reset run:\nwant: %#v\n got: %#v", expectedArguments, lastRun.Arguments)
	}
}

func TestContainerResetRejectsMissingSuperAdminGate(t *testing.T) {
	workDirectoryPath := prepareContainerRuntimeWorkDirectory(t)
	executor := &recordingContainerCommandExecutor{}
	runtime := ContainerRuntime{CommandExecutor: executor, Output: io.Discard, ErrorOutput: io.Discard}

	_, errorValue := runtime.Reset(ContainerResetOptions{WorkDirectoryPath: workDirectoryPath, Purge: true})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "--confirm-super-admin") {
		t.Fatalf("expected super-admin gate rejection, got %v", errorValue)
	}
	if len(executor.runs) != 0 {
		t.Fatalf("reset ran commands despite rejected gate: %+v", executor.runs)
	}
}

func prepareContainerRuntimeWorkDirectory(t *testing.T) string {
	t.Helper()
	workDirectoryPath := t.TempDir()
	if errorValue := os.MkdirAll(filepath.Join(workDirectoryPath, "secrets"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(workDirectoryPath, "secrets", "openrouter-key"), []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(workDirectoryPath, "tenants.generated.yml"), []byte("services: {}\n"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(workDirectoryPath, "cf.env"), []byte("CF_API_TOKEN=cf-token\nCF_ACCOUNT_ID=account-1\nCF_ZONE_ID=zone-1\nCF_TUNNEL_ID=tunnel-1\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return workDirectoryPath
}

func mustCreateContainerTenantDirectories(t *testing.T, workDirectoryPath string, runtimeID string) {
	t.Helper()
	for _, path := range []string{
		filepath.Join(workDirectoryPath, "config", runtimeID),
		filepath.Join(workDirectoryPath, "secrets", runtimeID),
	} {
		if errorValue := os.MkdirAll(path, 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if errorValue := os.WriteFile(filepath.Join(workDirectoryPath, "secrets", runtimeID, "db-password"), []byte("tenant_database_password_0123456789\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func mustContainerTenant(t *testing.T, index int) ContainerTenant {
	t.Helper()
	tenant, errorValue := ContainerTenantForIndex(index)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return tenant
}

func readContainerTestFile(t *testing.T, path string) string {
	t.Helper()
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(document)
}

func readContainerTestFileTrimmed(t *testing.T, path string) string {
	t.Helper()
	return strings.TrimSpace(readContainerTestFile(t, path))
}

func containsContainerArgumentSequence(arguments []string, sequence []string) bool {
	for index := 0; index+len(sequence) <= len(arguments); index++ {
		if slicesEqual(arguments[index:index+len(sequence)], sequence) {
			return true
		}
	}
	return false
}

func slicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index, value := range left {
		if value != right[index] {
			return false
		}
	}
	return true
}

func installContainerRuntimeHTTPTransport(t *testing.T) {
	t.Helper()
	originalTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = containerRuntimeHTTPTestTransport(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.Path == "/api/v4/users/login":
			return newContainerRuntimeHTTPResponse(t, http.StatusOK, map[string]string{"id": "admin-id"}, "mattermost-token"), nil
		case request.URL.Path == "/api/v4/users/username/internkim01":
			return newContainerRuntimeHTTPResponse(t, http.StatusOK, map[string]string{"id": "agent-id"}, ""), nil
		case request.URL.Path == "/api/v4/users/username/admin01":
			return newContainerRuntimeHTTPResponse(t, http.StatusOK, map[string]string{"id": "company-admin-id"}, ""), nil
		case request.URL.Path == "/api/v4/teams/name/tenant01":
			return newContainerRuntimeHTTPResponse(t, http.StatusOK, map[string]string{"id": "team-id"}, ""), nil
		case request.URL.Path == "/api/v4/users/agent-id/patch":
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["nickname"] != "김인턴" || payload["first_name"] != "Intern" || payload["last_name"] != "Kim" {
				t.Fatalf("unexpected agent profile patch: %#v", payload)
			}
			return newContainerRuntimeHTTPResponse(t, http.StatusOK, map[string]bool{"ok": true}, ""), nil
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
			if fileHeader.Filename != botassets.AvatarFileName() || !bytes.Equal(document, botassets.AvatarPNG()) {
				t.Fatalf("unexpected profile image upload: %s (%d bytes)", fileHeader.Filename, len(document))
			}
			return newContainerRuntimeHTTPResponse(t, http.StatusOK, map[string]bool{"ok": true}, ""), nil
		case request.URL.Path == "/api/v4/teams/team-id/members/company-admin-id/schemeRoles":
			return newContainerRuntimeHTTPResponse(t, http.StatusOK, map[string]bool{"ok": true}, ""), nil
		case request.URL.Path == "/api/v4/users/company-admin-id/patch":
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["nickname"] != "admin" {
				t.Fatalf("unexpected company admin profile patch: %#v", payload)
			}
			return newContainerRuntimeHTTPResponse(t, http.StatusOK, map[string]bool{"ok": true}, ""), nil
		case request.URL.Path == "/api/v4/channels/direct":
			return newContainerRuntimeHTTPResponse(t, http.StatusCreated, map[string]string{"id": "dm-channel-id"}, ""), nil
		case strings.Contains(request.URL.Path, "/accounts/account-1/cfd_tunnel/tunnel-1/configurations") && request.Method == http.MethodGet:
			return newContainerRuntimeHTTPResponse(t, http.StatusOK, cloudflareTunnelConfigurationResponse{Success: true, Result: cloudflareTunnelConfigurationResource{Configuration: cloudflareTunnelConfiguration{Ingress: []cloudflareTunnelIngress{{Hostname: "poc-0.intern.kim", Service: "http://mattermost:8065"}, {Service: "http_status:404"}}}}}, ""), nil
		case strings.Contains(request.URL.Path, "/accounts/account-1/cfd_tunnel/tunnel-1/configurations") && request.Method == http.MethodPut:
			return newContainerRuntimeHTTPResponse(t, http.StatusOK, map[string]bool{"success": true}, ""), nil
		case strings.Contains(request.URL.Path, "/zones/zone-1/dns_records") && request.Method == http.MethodGet:
			return newContainerRuntimeHTTPResponse(t, http.StatusOK, map[string]any{"success": true, "result": []any{}}, ""), nil
		case strings.Contains(request.URL.Path, "/zones/zone-1/dns_records") && (request.Method == http.MethodPost || request.Method == http.MethodDelete):
			return newContainerRuntimeHTTPResponse(t, http.StatusOK, map[string]bool{"success": true}, ""), nil
		default:
			t.Fatalf("unexpected HTTP request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultClient.Transport = originalTransport
	})
}

type containerRuntimeHTTPTestTransport func(request *http.Request) (*http.Response, error)

func (transport containerRuntimeHTTPTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func newContainerRuntimeHTTPResponse(t *testing.T, statusCode int, value any, token string) *http.Response {
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

func assertContainerRunArguments(t *testing.T, runs []ContainerCommandInvocation, expectedArguments [][]string) {
	t.Helper()
	if len(runs) != len(expectedArguments) {
		t.Fatalf("unexpected run count: want %d got %d\nruns: %+v", len(expectedArguments), len(runs), runs)
	}
	for index, expected := range expectedArguments {
		if runs[index].ExecutableName != "docker" {
			t.Fatalf("run %d used unexpected executable: %+v", index, runs[index])
		}
		if !reflect.DeepEqual(runs[index].Arguments, expected) {
			t.Fatalf("run %d arguments:\nwant: %#v\n got: %#v", index, expected, runs[index].Arguments)
		}
	}
}

func containerRunStatement(t *testing.T, invocation ContainerCommandInvocation) string {
	t.Helper()
	document, errorValue := io.ReadAll(invocation.Stdin)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(document)
}

func assertStringContains(t *testing.T, value string, fragment string) {
	t.Helper()
	if !strings.Contains(value, fragment) {
		t.Fatalf("value missing %q:\n%s", fragment, value)
	}
}

func copyContainerCommandInvocation(invocation ContainerCommandInvocation) ContainerCommandInvocation {
	copiedInvocation := invocation
	copiedInvocation.Arguments = append([]string{}, invocation.Arguments...)
	copiedInvocation.Environment = append([]string{}, invocation.Environment...)
	return copiedInvocation
}
