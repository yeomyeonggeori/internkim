package tenantruntime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestBootstrapTenantInstallsServicesAndDeviceTokenWithoutProviderKey(t *testing.T) {
	service := Service{BasePath: t.TempDir()}
	manifest := newTestCloudSharedManifest(t)
	if _, errorValue := service.CreateTenantFromTemplate(manifest, createTemplateRootFilesystem(t)); errorValue != nil {
		t.Fatal(errorValue)
	}

	status, errorValue := service.BootstrapTenant(manifest.TenantID, BootstrapOptions{
		BinaryDirectoryPath:  createTenantBinaryDirectory(t),
		GatewayURL:           "http://10.0.0.1:18081/api/v1/chat/completions",
		DeviceToken:          "device-token-acme",
		GatewaySharedSecret:  "gateway-secret-acme",
		ReleaseDownloadToken: "release-token-acme",
		AdminPassword:        "admin-password-acme",
		AdminEmail:           "admin@acme.test",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	paths, errorValue := BuildRuntimePaths(service.BasePath, manifest.TenantID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertFileContains(t, filepath.Join(paths.ContainerRootPath, "etc/systemd/system/internkim-capabilityd.service"), "After=network-online.target time-sync.target mattermost.service internkim-admind.service")
	assertFileContains(t, filepath.Join(paths.ContainerRootPath, "etc/systemd/system/internkim-capabilityd.service"), "Wants=network-online.target time-sync.target internkim-admind.service")
	assertFileContains(t, filepath.Join(paths.ContainerRootPath, "etc/systemd/system/internkim-capabilityd.service"), "--local-inference-mode remote")
	assertFileContains(t, filepath.Join(paths.ContainerRootPath, "etc/systemd/system/internkim-capabilityd.service"), "--openrouter-url http://10.0.0.1:18081/api/v1/chat/completions")
	assertFileContains(t, filepath.Join(paths.ContainerRootPath, "etc/systemd/system/internkim-capabilityd.service"), "--openrouter-key /root/.internkim/secrets/llm-device-token")
	assertFileContains(t, filepath.Join(paths.ContainerRootPath, "etc/systemd/system/internkim-capabilityd.service"), "--openrouter-gateway-secret /root/.internkim/secrets/llm-gateway-shared-secret")
	assertFileContains(t, filepath.Join(paths.ContainerRootPath, "etc/systemd/system/internkim-tenant-mattermost-firstboot.service"), "internkim-tenant-mattermost-firstboot.sh")
	assertFileContains(t, filepath.Join(paths.ContainerRootPath, "usr/local/bin/internkim-tenant-mattermost-firstboot.sh"), `\"username\":\"$admin_username\"`)
	assertFileContains(t, filepath.Join(paths.InternKimSecretsPath, "llm-device-token"), "device-token-acme")
	assertFileContains(t, filepath.Join(paths.InternKimSecretsPath, "llm-gateway-shared-secret"), "gateway-secret-acme")
	assertFileContains(t, filepath.Join(paths.InternKimSecretsPath, "release-download-token"), "release-token-acme")
	assertFileContains(t, filepath.Join(paths.InternKimSecretsPath, "mm-admin-pass"), "admin-password-acme")
	assertFileContains(t, filepath.Join(paths.InternKimPath, "config/admin-username"), "admin")
	if fileExists(filepath.Join(paths.InternKimSecretsPath, "openrouter-api-key")) {
		t.Fatal("bootstrap must not write an OpenRouter provider master key into tenant secrets")
	}
	assertFileContains(t, filepath.Join(paths.BlueclawRootPath, "config/policy.json"), "admin@acme.test")
	if !status.ContainerRootBootable {
		t.Fatalf("expected bootable status after bootstrap: %+v", status)
	}
}

func TestCreateCloudSharedFleetReturnsTenMattermostURLs(t *testing.T) {
	service := Service{BasePath: t.TempDir()}
	gatewayTokensPath := filepath.Join(t.TempDir(), "gateway-device-tokens.json")

	statuses, errorValue := service.CreateCloudSharedFleet(FleetCreateOptions{
		Count:               10,
		TenantPrefix:        "pilot",
		DisplayNamePrefix:   "PoC",
		AssignedHost:        "mac-a",
		PublicURLTemplate:   "https://{tenant}.mattermost.intern.test",
		MirrorHost:          "mac-b",
		GatewayTokensPath:   gatewayTokensPath,
		HardLimitMicrounits: 30000,
		RequestsPerMinute:   30,
		MattermostPortStart: 18065,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(statuses) != 10 {
		t.Fatalf("expected 10 tenant statuses, got %+v", statuses)
	}
	if statuses[0].MattermostURL != "https://pilot-01.mattermost.intern.test" {
		t.Fatalf("unexpected first Mattermost URL: %+v", statuses[0])
	}
	if statuses[9].MattermostURL != "https://pilot-10.mattermost.intern.test" {
		t.Fatalf("unexpected last Mattermost URL: %+v", statuses[9])
	}
	assertFleetCredentialsAreIssued(t, statuses)
	assertGatewayTokensMatchFleetCredentials(t, gatewayTokensPath, statuses)
	assertTenantMattermostInstancesAreSeparate(t, service.BasePath, statuses)
}

func TestCreateCloudSharedFleetRejectsSharedInstancePathURLs(t *testing.T) {
	service := Service{BasePath: t.TempDir()}

	_, errorValue := service.CreateCloudSharedFleet(FleetCreateOptions{
		Count:               10,
		TenantPrefix:        "pilot",
		DisplayNamePrefix:   "PoC",
		AssignedHost:        "mac-a",
		PublicURLTemplate:   "https://mattermost.intern.test/{tenant}",
		MattermostPortStart: 18065,
	})

	if errorValue == nil {
		t.Fatal("expected shared instance path URLs to be rejected")
	}
}

func TestCreateCloudSharedFleetCanAppendFromStartIndex(t *testing.T) {
	service := Service{BasePath: t.TempDir()}

	statuses, errorValue := service.CreateCloudSharedFleet(FleetCreateOptions{
		Count:               2,
		StartIndex:          11,
		TenantPrefix:        "pilot",
		DisplayNamePrefix:   "PoC",
		AssignedHost:        "mac-a",
		PublicURLTemplate:   "https://{tenant}.mattermost.intern.test",
		MattermostPortStart: 18075,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if statuses[0].TenantID != "pilot-11" || statuses[1].TenantID != "pilot-12" {
		t.Fatalf("expected appended tenant ids, got %+v", statuses)
	}
	manifest, errorValue := service.ReadManifest("pilot-11")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if manifest.MattermostInstance.Port != 18075 {
		t.Fatalf("expected first appended tenant to use requested port start, got %+v", manifest.MattermostInstance)
	}
}

func TestCreateCloudSharedFleetProvisionsOpenRouterKeys(t *testing.T) {
	gatewayTokensPath := filepath.Join(t.TempDir(), "gateway-device-tokens.json")
	provisioner := &fakeOpenRouterKeyProvisioner{}
	service := Service{
		BasePath:                 t.TempDir(),
		OpenRouterKeyProvisioner: provisioner,
	}

	statuses, errorValue := service.CreateCloudSharedFleet(FleetCreateOptions{
		Count:                       2,
		TenantPrefix:                "pilot",
		DisplayNamePrefix:           "PoC",
		AssignedHost:                "mac-a",
		PublicURLTemplate:           "https://{tenant}.mattermost.intern.test",
		GatewayTokensPath:           gatewayTokensPath,
		HardLimitMicrounits:         30000,
		RequestsPerMinute:           30,
		MattermostPortStart:         18065,
		OpenRouterManagementKeyPath: filepath.Join(t.TempDir(), "management-key"),
		OpenRouterKeyLimitUSD:       20,
		OpenRouterKeyLimitReset:     "monthly",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(provisioner.Requests) != 2 {
		t.Fatalf("expected two provisioned OpenRouter keys, got %+v", provisioner.Requests)
	}
	if provisioner.Requests[0].Name != "internkim-pilot-01" || provisioner.Requests[0].LimitUSD != 20 || provisioner.Requests[0].LimitReset != "monthly" {
		t.Fatalf("unexpected first OpenRouter key request: %+v", provisioner.Requests[0])
	}
	if statuses[0].ProviderOpenRouterAPIKey != "sk-or-v1-internkim-pilot-01" || statuses[1].ProviderOpenRouterAPIKey != "sk-or-v1-internkim-pilot-02" {
		t.Fatalf("expected provider OpenRouter keys in fleet status, got %+v", statuses)
	}
	assertGatewayTokensContainProviderKeys(t, gatewayTokensPath, statuses)
}

func TestCreateCloudSharedFleetRequiresGatewayTokensWhenProvisioningOpenRouterKeys(t *testing.T) {
	service := Service{BasePath: t.TempDir(), OpenRouterKeyProvisioner: &fakeOpenRouterKeyProvisioner{}}

	_, errorValue := service.CreateCloudSharedFleet(FleetCreateOptions{
		Count:                       1,
		TenantPrefix:                "pilot",
		DisplayNamePrefix:           "PoC",
		AssignedHost:                "mac-a",
		PublicURLTemplate:           "https://{tenant}.mattermost.intern.test",
		MattermostPortStart:         18065,
		OpenRouterManagementKeyPath: "management-key",
	})

	if errorValue == nil {
		t.Fatal("expected gateway token path requirement when OpenRouter keys are provisioned")
	}
}

func assertFleetCredentialsAreIssued(t *testing.T, statuses []FleetTenantStatus) {
	t.Helper()
	openRouterAPIKeys := map[string]bool{}
	adminPasswords := map[string]bool{}
	for _, status := range statuses {
		if status.AdminUsername != TenantInitialAdminUsername {
			t.Fatalf("expected admin username %q, got %+v", TenantInitialAdminUsername, status)
		}
		if status.OpenRouterAPIKey == "" || status.AdminPassword == "" {
			t.Fatalf("expected issued credentials, got %+v", status)
		}
		if openRouterAPIKeys[status.OpenRouterAPIKey] {
			t.Fatalf("expected unique OpenRouter-compatible API keys, duplicate in %+v", statuses)
		}
		if adminPasswords[status.AdminPassword] {
			t.Fatalf("expected unique admin passwords, duplicate in %+v", statuses)
		}
		openRouterAPIKeys[status.OpenRouterAPIKey] = true
		adminPasswords[status.AdminPassword] = true
	}
}

func assertGatewayTokensContainProviderKeys(t *testing.T, path string, statuses []FleetTenantStatus) {
	t.Helper()
	documentBytes, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var document struct {
		DeviceTokens []struct {
			Token          string `json:"token"`
			ProviderAPIKey string `json:"providerAPIKey"`
		} `json:"deviceTokens"`
	}
	if errorValue := json.Unmarshal(documentBytes, &document); errorValue != nil {
		t.Fatal(errorValue)
	}
	for index, status := range statuses {
		token := document.DeviceTokens[index]
		if token.Token != status.OpenRouterAPIKey || token.ProviderAPIKey != status.ProviderOpenRouterAPIKey {
			t.Fatalf("gateway provider key does not match fleet status: token=%+v status=%+v", token, status)
		}
	}
}

func assertTenantMattermostInstancesAreSeparate(t *testing.T, basePath string, statuses []FleetTenantStatus) {
	t.Helper()
	ports := map[int]bool{}
	databaseNames := map[string]bool{}
	for _, status := range statuses {
		manifest, errorValue := (Service{BasePath: basePath}).ReadManifest(status.TenantID)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		instance := manifest.MattermostInstance
		if instance.PublicURL != status.MattermostURL || instance.Port <= 0 || instance.DatabaseName == "" {
			t.Fatalf("expected tenant mattermost instance metadata, got %+v", manifest)
		}
		if ports[instance.Port] {
			t.Fatalf("expected unique Mattermost ports, duplicate in %+v", instance)
		}
		if databaseNames[instance.DatabaseName] {
			t.Fatalf("expected unique Mattermost databases, duplicate in %+v", instance)
		}
		ports[instance.Port] = true
		databaseNames[instance.DatabaseName] = true
	}
}

func assertGatewayTokensMatchFleetCredentials(t *testing.T, path string, statuses []FleetTenantStatus) {
	t.Helper()
	documentBytes, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var document struct {
		DeviceTokens []struct {
			Token               string `json:"token"`
			TokenHash           string `json:"tokenHash"`
			TenantID            string `json:"tenantID"`
			DeviceID            string `json:"deviceID"`
			HardLimitMicrounits int64  `json:"hardLimitMicrounits"`
			RequestsPerMinute   int    `json:"requestsPerMinute"`
		} `json:"deviceTokens"`
	}
	if errorValue := json.Unmarshal(documentBytes, &document); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(document.DeviceTokens) != len(statuses) {
		t.Fatalf("expected %d gateway tokens, got %+v", len(statuses), document)
	}
	for index, status := range statuses {
		token := document.DeviceTokens[index]
		if token.Token != status.OpenRouterAPIKey || token.TokenHash == "" || token.TenantID != status.TenantID || token.DeviceID == "" || token.HardLimitMicrounits != 30000 || token.RequestsPerMinute != 30 {
			t.Fatalf("gateway token does not match fleet status: token=%+v status=%+v", token, status)
		}
	}
}

func createTenantBinaryDirectory(t *testing.T) string {
	t.Helper()
	binaryDirectoryPath := t.TempDir()
	for _, binary := range tenantRequiredBinaries() {
		path := filepath.Join(binaryDirectoryPath, binary.name)
		if errorValue := os.WriteFile(path, []byte(binary.name), 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	return binaryDirectoryPath
}

func assertFileContains(t *testing.T, path string, expectedText string) {
	t.Helper()
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(document), expectedText) {
		t.Fatalf("expected %s to contain %q, got:\n%s", path, expectedText, string(document))
	}
}

func TestTenantRequiredBinariesIncludeCloudSharedServices(t *testing.T) {
	names := []string{}
	for _, binary := range tenantRequiredBinaries() {
		names = append(names, binary.name)
	}
	for _, expectedName := range []string{
		blueclaw.CapabilitydName,
		blueclaw.AdmindName,
		blueclaw.BlueclawName,
		blueclaw.BlueclawSupervisorName,
		blueclaw.GraphitiMemorydName,
	} {
		if !containsString(names, expectedName) {
			t.Fatalf("expected required tenant binaries to include %q, got %+v", expectedName, names)
		}
	}
}

func containsString(values []string, expectedValue string) bool {
	for _, value := range values {
		if value == expectedValue {
			return true
		}
	}
	return false
}

type fakeOpenRouterKeyProvisioner struct {
	Requests []OpenRouterAPIKeyCreateRequest
}

func (provisioner *fakeOpenRouterKeyProvisioner) CreateAPIKey(ctx context.Context, request OpenRouterAPIKeyCreateRequest) (OpenRouterProvisionedAPIKey, error) {
	_ = ctx
	provisioner.Requests = append(provisioner.Requests, request)
	return OpenRouterProvisionedAPIKey{
		APIKey: "sk-or-v1-" + request.Name,
		Hash:   "hash-" + request.Name,
		Label:  "label-" + request.Name,
	}, nil
}
