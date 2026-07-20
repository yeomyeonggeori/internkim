package tenantruntime

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const tenantDeviceTokenPath = "/root/.internkim/secrets/llm-device-token"
const tenantGatewaySecretPath = "/root/.internkim/secrets/llm-gateway-shared-secret"
const tenantReleaseDownloadTokenPath = "/root/.internkim/secrets/release-download-token"
const tenantMattermostFirstBootScriptPath = "/usr/local/bin/internkim-tenant-mattermost-firstboot.sh"
const tenantMattermostFirstBootServicePath = "/etc/systemd/system/internkim-tenant-mattermost-firstboot.service"

type BootstrapOptions struct {
	BinaryDirectoryPath  string
	GatewayURL           string
	DeviceToken          string
	GatewaySharedSecret  string
	ReleaseDownloadToken string
	AdminPassword        string
	AdminEmail           string
	ModelName            string
}

func (service Service) BootstrapTenant(tenantID string, options BootstrapOptions) (TenantStatus, error) {
	manifest, paths, errorValue := service.cloudSharedTenant(tenantID)
	if errorValue != nil {
		return TenantStatus{}, errorValue
	}
	if !bootableRootFilesystemExists(paths.ContainerRootPath) {
		return TenantStatus{}, errors.New("tenant rootfs is not bootable: expected systemd or init inside rootfs")
	}
	if errorValue := validateBootstrapOptions(options); errorValue != nil {
		return TenantStatus{}, errorValue
	}
	options, errorValue = optionsWithGeneratedCredentials(options)
	if errorValue != nil {
		return TenantStatus{}, errorValue
	}
	if errorValue := installTenantBinaries(paths, options.BinaryDirectoryPath); errorValue != nil {
		return TenantStatus{}, errorValue
	}
	if errorValue := installTenantServiceUnits(paths, options); errorValue != nil {
		return TenantStatus{}, errorValue
	}
	if errorValue := installTenantRuntimeConfiguration(paths, options); errorValue != nil {
		return TenantStatus{}, errorValue
	}
	return service.Status(manifest.TenantID)
}

func validateBootstrapOptions(options BootstrapOptions) error {
	if strings.TrimSpace(options.BinaryDirectoryPath) == "" {
		return errors.New("binary directory path is required")
	}
	if strings.TrimSpace(options.GatewayURL) == "" {
		return errors.New("gateway URL is required")
	}
	return nil
}

func optionsWithGeneratedCredentials(options BootstrapOptions) (BootstrapOptions, error) {
	if strings.TrimSpace(options.DeviceToken) != "" && strings.TrimSpace(options.AdminPassword) != "" {
		return options, nil
	}
	credentials, errorValue := GenerateTenantCredentials()
	if errorValue != nil {
		return BootstrapOptions{}, errorValue
	}
	if strings.TrimSpace(options.DeviceToken) == "" {
		options.DeviceToken = credentials.OpenRouterAPIKey
	}
	if strings.TrimSpace(options.AdminPassword) == "" {
		options.AdminPassword = credentials.AdminPassword
	}
	return options, nil
}

func installTenantBinaries(paths RuntimePaths, binaryDirectoryPath string) error {
	for _, binary := range tenantRequiredBinaries() {
		sourcePath := filepath.Join(filepath.Clean(binaryDirectoryPath), binary.name)
		destinationPath := filepath.Join(paths.ContainerRootPath, strings.TrimPrefix(binary.path, "/"))
		if !fileExists(sourcePath) {
			return errors.New("required tenant binary is missing: " + sourcePath)
		}
		if errorValue := copyRegularFile(sourcePath, destinationPath, 0o755); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func tenantRequiredBinaries() []struct {
	name string
	path string
} {
	return []struct {
		name string
		path string
	}{
		{name: blueclaw.CapabilitydName, path: blueclaw.CapabilitydBinaryPath},
		{name: blueclaw.AdmindName, path: blueclaw.AdmindBinaryPath},
		{name: blueclaw.BlueclawName, path: blueclaw.BlueclawBinaryPath},
		{name: blueclaw.BlueclawSupervisorName, path: blueclaw.BlueclawSupervisorBinaryPath},
		{name: blueclaw.GraphitiMemorydName, path: blueclaw.GraphitiMemorydPath},
	}
}

func installTenantServiceUnits(paths RuntimePaths, options BootstrapOptions) error {
	serviceUnits := []struct {
		path     string
		document string
	}{
		{path: blueclaw.BlueclawServicePath, document: blueclaw.BlueclawServiceUnit()},
		{path: blueclaw.CapabilitydServicePath, document: tenantCapabilitydServiceUnit(options)},
		{path: blueclaw.GraphitiMemorydServicePath, document: blueclaw.GraphitiMemorydServiceUnit()},
		{path: blueclaw.AdmindServicePath, document: blueclaw.AdmindServiceUnit()},
		{path: tenantMattermostFirstBootServicePath, document: tenantMattermostFirstBootServiceUnit()},
	}
	for _, serviceUnit := range serviceUnits {
		rootfsPath := filepath.Join(paths.ContainerRootPath, strings.TrimPrefix(serviceUnit.path, "/"))
		if errorValue := os.MkdirAll(filepath.Dir(rootfsPath), 0o755); errorValue != nil {
			return errorValue
		}
		if errorValue := os.WriteFile(rootfsPath, []byte(serviceUnit.document), 0o644); errorValue != nil {
			return errorValue
		}
		if errorValue := enableTenantService(paths, filepath.Base(serviceUnit.path)); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func enableTenantService(paths RuntimePaths, serviceName string) error {
	wantsPath := filepath.Join(paths.ContainerRootPath, "etc/systemd/system/multi-user.target.wants")
	if errorValue := os.MkdirAll(wantsPath, 0o755); errorValue != nil {
		return errorValue
	}
	linkPath := filepath.Join(wantsPath, serviceName)
	if fileExists(linkPath) {
		if errorValue := os.Remove(linkPath); errorValue != nil {
			return errorValue
		}
	}
	return os.Symlink("../"+serviceName, linkPath)
}

func tenantCapabilitydServiceUnit(options BootstrapOptions) string {
	command := blueclaw.CapabilitydBinaryPath +
		" --vsock-port " + intString(blueclaw.CapabilityVSockPort) +
		" --companion-url http://127.0.0.1:18080/_internkim/companion" +
		" --mattermost-url " + blueclaw.BlueclawMattermostLocalURL +
		" --mattermost-token " + blueclaw.BlueclawMattermostTokenPath +
		" --local-inference-mode remote" +
		" --openrouter-url " + strings.TrimSpace(options.GatewayURL) +
		" --openrouter-key " + tenantDeviceTokenPath +
		optionalGatewaySecretArgument(options)
	return `[Unit]
Description=InternKim Capability Daemon
After=network-online.target time-sync.target mattermost.service internkim-admind.service
Wants=network-online.target time-sync.target internkim-admind.service

[Service]
User=root
RuntimeDirectory=internkim
ExecStart=` + command + `
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`
}

func optionalGatewaySecretArgument(options BootstrapOptions) string {
	if strings.TrimSpace(options.GatewaySharedSecret) == "" {
		return ""
	}
	return " --openrouter-gateway-secret " + tenantGatewaySecretPath
}

func installTenantRuntimeConfiguration(paths RuntimePaths, options BootstrapOptions) error {
	if errorValue := createRuntimeDirectories(paths); errorValue != nil {
		return errorValue
	}
	runtimeConfiguration, errorValue := blueclaw.BlueclawRuntimeConfigDocument(options.ModelName)
	if errorValue != nil {
		return errorValue
	}
	policyDocument, errorValue := blueclaw.BlueclawPolicyDocument(options.AdminEmail)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := writeTenantFile(paths.BlueclawRootPath, "config/runtime.json", runtimeConfiguration, 0o640); errorValue != nil {
		return errorValue
	}
	if errorValue := writeTenantFile(paths.BlueclawRootPath, "config/policy.json", policyDocument, 0o640); errorValue != nil {
		return errorValue
	}
	if errorValue := writeTenantFile(paths.BlueclawWorkspacePath, ".blueclaw/config/runtime.json", runtimeConfiguration, 0o640); errorValue != nil {
		return errorValue
	}
	if errorValue := writeTenantFile(paths.BlueclawWorkspacePath, ".blueclaw/config/policy.json", policyDocument, 0o640); errorValue != nil {
		return errorValue
	}
	credentials := TenantCredentials{
		OpenRouterAPIKey: strings.TrimSpace(options.DeviceToken),
		AdminUsername:    TenantInitialAdminUsername,
		AdminPassword:    strings.TrimSpace(options.AdminPassword),
	}
	if errorValue := installTenantCredentials(paths, credentials); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(options.GatewaySharedSecret) != "" {
		if errorValue := writeTenantFile(paths.InternKimPath, "secrets/llm-gateway-shared-secret", strings.TrimSpace(options.GatewaySharedSecret), 0o600); errorValue != nil {
			return errorValue
		}
	}
	if strings.TrimSpace(options.ReleaseDownloadToken) != "" {
		if errorValue := writeTenantFile(paths.InternKimPath, "secrets/release-download-token", strings.TrimSpace(options.ReleaseDownloadToken), 0o600); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := writeTenantFile(paths.InternKimPath, "config/admin-email", tenantAdminEmail(paths, options), 0o640); errorValue != nil {
		return errorValue
	}
	if errorValue := writeTenantFile(paths.InternKimPath, "env/mattermost-url", "http://127.0.0.1:8065", 0o640); errorValue != nil {
		return errorValue
	}
	if errorValue := installTenantMattermostFirstBootScript(paths); errorValue != nil {
		return errorValue
	}
	for _, directory := range []string{
		".blueclaw/postgres",
		".blueclaw/graphiti",
		".blueclaw/logs",
		".blueclaw/blobs",
		"skills",
		"bin",
		"downloads",
	} {
		if errorValue := os.MkdirAll(filepath.Join(paths.BlueclawWorkspacePath, directory), 0o750); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func installTenantCredentials(paths RuntimePaths, credentials TenantCredentials) error {
	if strings.TrimSpace(credentials.OpenRouterAPIKey) == "" {
		return errors.New("tenant OpenRouter-compatible API key is required")
	}
	if strings.TrimSpace(credentials.AdminPassword) == "" {
		return errors.New("tenant admin password is required")
	}
	if errorValue := writeTenantFile(paths.InternKimPath, "secrets/llm-device-token", strings.TrimSpace(credentials.OpenRouterAPIKey), 0o600); errorValue != nil {
		return errorValue
	}
	if errorValue := writeTenantFile(paths.InternKimPath, "secrets/mm-admin-pass", strings.TrimSpace(credentials.AdminPassword), 0o600); errorValue != nil {
		return errorValue
	}
	return writeTenantFile(paths.InternKimPath, "config/admin-username", TenantInitialAdminUsername, 0o640)
}

func tenantAdminEmail(paths RuntimePaths, options BootstrapOptions) string {
	if strings.TrimSpace(options.AdminEmail) != "" {
		return strings.TrimSpace(options.AdminEmail)
	}
	return DefaultTenantAdminEmail(filepath.Base(paths.TenantRootPath))
}

func DefaultTenantAdminEmail(tenantID string) string {
	return TenantInitialAdminUsername + "@" + strings.TrimSpace(tenantID) + ".local"
}

func installTenantMattermostFirstBootScript(paths RuntimePaths) error {
	rootfsPath := filepath.Join(paths.ContainerRootPath, strings.TrimPrefix(tenantMattermostFirstBootScriptPath, "/"))
	if errorValue := os.MkdirAll(filepath.Dir(rootfsPath), 0o755); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(rootfsPath, []byte(tenantMattermostFirstBootScript()), 0o755)
}

func tenantMattermostFirstBootServiceUnit() string {
	return `[Unit]
Description=InternKim tenant Mattermost first boot
After=network-online.target mattermost.service
Wants=network-online.target mattermost.service

[Service]
Type=oneshot
ExecStart=` + tenantMattermostFirstBootScriptPath + `
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
`
}

func tenantMattermostFirstBootScript() string {
	return `#!/bin/sh
set -eu
marker="/root/.internkim/state/mattermost-admin-created"
if [ -f "$marker" ]; then
  exit 0
fi
admin_username="$(cat /root/.internkim/config/admin-username)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
admin_email="$(cat /root/.internkim/config/admin-email)"
for attempt in $(seq 1 120); do
  if curl -fsS http://127.0.0.1:8065/api/v4/system/ping >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
status="$(curl -sS -o /tmp/internkim-tenant-admin-create.json -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$admin_email\",\"username\":\"$admin_username\",\"password\":\"$admin_password\"}" \
  http://127.0.0.1:8065/api/v4/users || true)"
if [ "$status" != "201" ] && [ "$status" != "400" ]; then
  cat /tmp/internkim-tenant-admin-create.json >&2 || true
  exit 1
fi
mkdir -p /root/.internkim/state
date -u +%FT%TZ > "$marker"
`
}

func writeTenantFile(rootPath string, relativePath string, document string, mode os.FileMode) error {
	path := filepath.Join(rootPath, relativePath)
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o750); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, []byte(document), mode)
}

func intString(value int) string {
	return strconv.Itoa(value)
}
