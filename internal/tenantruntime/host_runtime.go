package tenantruntime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type HostRuntimeOptions struct {
	GatewayURL                 string
	GatewaySharedSecret        string
	ReleaseDownloadToken       string
	ModelName                  string
	RootFilesystemTemplatePath string
	WorkspaceImageTemplatePath string
	GraphitiPackagePath        string
	GraphitiVirtualEnvPath     string
	RuntimeDirectoryBasePath   string
	PortBase                   int
}

type HostRuntimeStatus struct {
	TenantID                  string            `json:"tenantID"`
	MattermostURL             string            `json:"mattermostURL"`
	AdmindURL                 string            `json:"admindURL"`
	BlueclawURL               string            `json:"blueclawURL"`
	GraphitiURL               string            `json:"graphitiURL"`
	ServiceNames              map[string]string `json:"serviceNames"`
	RuntimeConfigurationPath  string            `json:"runtimeConfigurationPath"`
	RootFilesystemImagePath   string            `json:"rootFilesystemImagePath"`
	WorkspaceImagePath        string            `json:"workspaceImagePath"`
	GraphitiVirtualEnvPath    string            `json:"graphitiVirtualEnvPath"`
	MattermostBotTokenReady   bool              `json:"mattermostBotTokenReady"`
	LLMDeviceTokenReady       bool              `json:"llmDeviceTokenReady"`
	GatewaySharedSecretReady  bool              `json:"gatewaySharedSecretReady"`
	GraphitiReady             bool              `json:"graphitiReady"`
	RuntimeConfigurationReady bool              `json:"runtimeConfigurationReady"`
}

func (service Service) InstallHostRuntime(ctx context.Context, tenantID string, options HostRuntimeOptions) (HostRuntimeStatus, error) {
	manifest, paths, errorValue := service.cloudSharedTenant(tenantID)
	if errorValue != nil {
		return HostRuntimeStatus{}, errorValue
	}
	configuration := hostRuntimeConfiguration(manifest, paths, options)
	if errorValue := validateHostRuntimeConfiguration(configuration); errorValue != nil {
		return HostRuntimeStatus{}, errorValue
	}
	if errorValue := service.stopHostRuntimeBlueclaw(ctx, manifest); errorValue != nil {
		return HostRuntimeStatus{}, errorValue
	}
	if errorValue := installHostRuntimeFiles(manifest, paths, configuration); errorValue != nil {
		return HostRuntimeStatus{}, errorValue
	}
	if errorValue := service.patchHostRuntimeRootFilesystemImage(ctx, configuration); errorValue != nil {
		return HostRuntimeStatus{}, errorValue
	}
	if errorValue := service.ensureHostGraphitiDependencies(ctx, configuration); errorValue != nil {
		return HostRuntimeStatus{}, errorValue
	}
	if errorValue := service.installHostRuntimeUnits(ctx, manifest, paths, configuration); errorValue != nil {
		return HostRuntimeStatus{}, errorValue
	}
	return hostRuntimeStatus(manifest, paths, configuration), nil
}

func hostRuntimeConfiguration(manifest Manifest, paths RuntimePaths, options HostRuntimeOptions) hostRuntimeConfigurationDocument {
	tenantIndex := tenantNumericSuffix(manifest.TenantID)
	portBase := firstPositiveTenantInt(options.PortBase, defaultTenantHostRuntimePortBase(tenantIndex))
	return hostRuntimeConfigurationDocument{
		GatewayURL:                 strings.TrimSpace(options.GatewayURL),
		GatewaySharedSecret:        strings.TrimSpace(options.GatewaySharedSecret),
		ReleaseDownloadToken:       strings.TrimSpace(options.ReleaseDownloadToken),
		ModelName:                  strings.TrimSpace(options.ModelName),
		RootFilesystemTemplatePath: firstNonEmptyTenantString(options.RootFilesystemTemplatePath, blueclaw.BlueclawRootFilesystemImagePath),
		WorkspaceImageTemplatePath: firstNonEmptyTenantString(options.WorkspaceImageTemplatePath, blueclaw.BlueclawWorkspaceImagePath),
		GraphitiPackagePath:        firstNonEmptyTenantString(options.GraphitiPackagePath, blueclaw.GraphitiMemorydPackagePath),
		GraphitiVirtualEnvPath:     firstNonEmptyTenantString(options.GraphitiVirtualEnvPath, "/opt/internkim/graphiti-venv"),
		RuntimeDirectoryBasePath:   firstNonEmptyTenantString(options.RuntimeDirectoryBasePath, "/srv/ikrt"),
		BlueclawPort:               portBase,
		AdmindPort:                 portBase + 80,
		GraphitiPort:               portBase + 91,
		CapabilityVSockPort:        blueclaw.CapabilityVSockPort,
		HealthPort:                 8082,
		GuestHTTPPort:              8081,
		BridgePort:                 portBase + 778,
		NetworkIndex:               100 + firstPositiveTenantInt(tenantIndex, 1),
		RootFilesystemImagePath:    filepath.Join(paths.BlueclawRootPath, "firecracker", "rootfs.ext4"),
		WorkspaceImagePath:         filepath.Join(paths.BlueclawRootPath, "firecracker", "workspace.ext4"),
	}
}

type hostRuntimeConfigurationDocument struct {
	GatewayURL                 string
	GatewaySharedSecret        string
	ReleaseDownloadToken       string
	ModelName                  string
	RootFilesystemTemplatePath string
	WorkspaceImageTemplatePath string
	GraphitiPackagePath        string
	GraphitiVirtualEnvPath     string
	RuntimeDirectoryBasePath   string
	BlueclawPort               int
	AdmindPort                 int
	GraphitiPort               int
	CapabilityVSockPort        int
	HealthPort                 int
	GuestHTTPPort              int
	BridgePort                 int
	NetworkIndex               int
	RootFilesystemImagePath    string
	WorkspaceImagePath         string
}

func validateHostRuntimeConfiguration(configuration hostRuntimeConfigurationDocument) error {
	if strings.TrimSpace(configuration.GatewayURL) == "" {
		return errors.New("gateway URL is required")
	}
	if configuration.BlueclawPort <= 0 || configuration.AdmindPort <= 0 || configuration.GraphitiPort <= 0 || configuration.CapabilityVSockPort <= 0 {
		return errors.New("host runtime ports must be positive")
	}
	if !fileExists(filepath.Join(configuration.GraphitiPackagePath, "requirements.txt")) {
		return errors.New("Graphiti memory daemon requirements are missing: " + filepath.Join(configuration.GraphitiPackagePath, "requirements.txt"))
	}
	return nil
}

func installHostRuntimeFiles(manifest Manifest, paths RuntimePaths, configuration hostRuntimeConfigurationDocument) error {
	if errorValue := createRuntimeDirectories(paths); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Join(paths.InternKimPath, "run"), 0o700); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(hostRuntimeDirectoryPath(manifest, configuration), 0o700); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(configuration.GatewaySharedSecret) != "" {
		if errorValue := writeTenantFile(paths.InternKimPath, "secrets/llm-gateway-shared-secret", configuration.GatewaySharedSecret, 0o600); errorValue != nil {
			return errorValue
		}
	}
	if strings.TrimSpace(configuration.ReleaseDownloadToken) != "" {
		if errorValue := writeTenantFile(paths.InternKimPath, "secrets/release-download-token", configuration.ReleaseDownloadToken, 0o600); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := writeTenantFile(paths.InternKimPath, "env/device-url", manifest.PublicURL, 0o640); errorValue != nil {
		return errorValue
	}
	if errorValue := writeTenantFile(paths.InternKimPath, "env/fleet-id", manifest.AssignedHost, 0o640); errorValue != nil {
		return errorValue
	}
	if errorValue := writeTenantFile(paths.InternKimPath, "config/admin-email", DefaultTenantAdminEmail(manifest.TenantID), 0o640); errorValue != nil {
		return errorValue
	}
	if errorValue := copyRegularFile(configuration.RootFilesystemTemplatePath, configuration.RootFilesystemImagePath, 0o600); errorValue != nil {
		return errorValue
	}
	if errorValue := copyRegularFile(configuration.WorkspaceImageTemplatePath, configuration.WorkspaceImagePath, 0o600); errorValue != nil {
		return errorValue
	}
	return os.MkdirAll(filepath.Join(paths.BlueclawRootPath, "logs", "supervisor"), 0o750)
}

func (service Service) patchHostRuntimeRootFilesystemImage(ctx context.Context, configuration hostRuntimeConfigurationDocument) error {
	_, errorValue := service.commandRunner().Run(ctx, ExecutableCommand{
		ExecutableName: "sh",
		Arguments:      []string{"-c", hostRuntimeRootFilesystemPatchScript(configuration)},
	})
	return errorValue
}

func hostRuntimeRootFilesystemPatchScript(configuration hostRuntimeConfigurationDocument) string {
	return strings.Join([]string{
		"set -eu",
		tenantE2fsckFunction(),
		"image_path=" + quoteShellArgument(configuration.RootFilesystemImagePath),
		"mount_path=\"$(mktemp -d)\"",
		"cleanup() {",
		"  if mountpoint -q \"$mount_path\"; then umount \"$mount_path\"; fi",
		"  rmdir \"$mount_path\"",
		"}",
		"trap cleanup EXIT",
		"tenant_e2fsck \"$image_path\"",
		"mount -o loop \"$image_path\" \"$mount_path\"",
		"sed -i 's/-target-tcp 127\\.0\\.0\\.1:8080/-target-tcp 127.0.0.1:" + intString(configuration.BlueclawPort) + "/g' \"$mount_path/sbin/init\"",
		"umount \"$mount_path\"",
		"rmdir \"$mount_path\"",
		"trap - EXIT",
		"tenant_e2fsck \"$image_path\"",
	}, "\n")
}

func (service Service) installHostRuntimeUnits(ctx context.Context, manifest Manifest, paths RuntimePaths, configuration hostRuntimeConfigurationDocument) error {
	runtimeConfiguration, errorValue := tenantHostBlueclawRuntimeConfiguration(manifest, paths, configuration)
	if errorValue != nil {
		return errorValue
	}
	policyDocument, errorValue := blueclaw.BlueclawPolicyDocument(DefaultTenantAdminEmail(manifest.TenantID))
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
	if errorValue := service.stopHostRuntimeBlueclaw(ctx, manifest); errorValue != nil {
		return errorValue
	}
	if errorValue := service.repairHostRuntimeWorkspaceImage(ctx, configuration); errorValue != nil {
		return errorValue
	}
	if errorValue := service.syncHostRuntimeWorkspaceImage(ctx, paths, configuration); errorValue != nil {
		return errorValue
	}
	if errorValue := writeHostRuntimeSystemdUnits(service.systemdSystemDirectoryPath(), manifest, paths, configuration); errorValue != nil {
		return errorValue
	}
	commands := []ExecutableCommand{
		{ExecutableName: "systemctl", Arguments: []string{"daemon-reload"}},
	}
	for _, serviceName := range hostRuntimeServiceNames(manifest) {
		commands = append(commands,
			ExecutableCommand{ExecutableName: "systemctl", Arguments: []string{"enable", serviceName}},
			ExecutableCommand{ExecutableName: "systemctl", Arguments: []string{"restart", serviceName}},
		)
	}
	for _, command := range commands {
		if _, errorValue := service.commandRunner().Run(ctx, command); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service Service) stopHostRuntimeBlueclaw(ctx context.Context, manifest Manifest) error {
	_, errorValue := service.commandRunner().Run(ctx, ExecutableCommand{
		ExecutableName: "sh",
		Arguments:      []string{"-c", "systemctl stop " + quoteShellArgument(hostRuntimeBlueclawServiceName(manifest)) + " 2>/dev/null || true"},
	})
	return errorValue
}

func (service Service) repairHostRuntimeWorkspaceImage(ctx context.Context, configuration hostRuntimeConfigurationDocument) error {
	command := strings.Join([]string{
		"set -eu",
		tenantE2fsckFunction(),
		"mount_path=\"$(mktemp -d)\"",
		"cleanup() {",
		"  if mountpoint -q \"$mount_path\"; then umount \"$mount_path\"; fi",
		"  rmdir \"$mount_path\"",
		"}",
		"trap cleanup EXIT",
		"tenant_e2fsck " + quoteShellArgument(configuration.WorkspaceImagePath),
		"mount -o loop " + quoteShellArgument(configuration.WorkspaceImagePath) + " \"$mount_path\"",
		"rm -rf \"$mount_path/.blueclaw/postgres/data\"",
		"rm -f \"$mount_path/.blueclaw/postgres/.s.PGSQL.5432\" \"$mount_path/.blueclaw/postgres/.s.PGSQL.5432.lock\"",
		"rm -rf \"$mount_path/.blueclaw/graphiti/kuzu\" \"$mount_path/.blueclaw/graphiti/kuzu.wal\"",
		"mkdir -p \"$mount_path/.blueclaw/postgres/data\" \"$mount_path/.blueclaw/graphiti\"",
		"umount \"$mount_path\"",
		"rmdir \"$mount_path\"",
		"trap - EXIT",
		"tenant_e2fsck " + quoteShellArgument(configuration.WorkspaceImagePath),
	}, "\n")
	_, errorValue := service.commandRunner().Run(ctx, ExecutableCommand{
		ExecutableName: "sh",
		Arguments:      []string{"-c", command},
	})
	return errorValue
}

func tenantE2fsckFunction() string {
	return strings.Join([]string{
		"tenant_e2fsck() {",
		"  set +e",
		"  e2fsck -fy \"$1\" >/dev/null",
		"  status=\"$?\"",
		"  set -e",
		"  [ \"$status\" = 0 ] || [ \"$status\" = 1 ]",
		"}",
	}, "\n")
}

func (service Service) syncHostRuntimeWorkspaceImage(ctx context.Context, paths RuntimePaths, configuration hostRuntimeConfigurationDocument) error {
	_, syncError := service.commandRunner().Run(ctx, ExecutableCommand{
		ExecutableName: blueclaw.BlueclawSupervisorBinaryPath,
		Arguments: []string{
			"sync-workspace",
			"--atomic",
			"--preserve-guest-state",
			"--workspace-image", configuration.WorkspaceImagePath,
			"--source", paths.BlueclawWorkspacePath,
		},
	})
	return syncError
}

func (service Service) ensureHostGraphitiDependencies(ctx context.Context, configuration hostRuntimeConfigurationDocument) error {
	pythonPath := filepath.Join(configuration.GraphitiVirtualEnvPath, "bin", "python")
	if fileExists(pythonPath) {
		return nil
	}
	command := strings.Join([]string{
		"set -eu",
		"if ! command -v uv >/dev/null 2>&1; then",
		"  curl -LsSf https://astral.sh/uv/0.11.11/install.sh -o /tmp/internkim-uv-install.sh",
		"  UV_UNMANAGED_INSTALL=/usr/local/bin sh /tmp/internkim-uv-install.sh",
		"fi",
		"uv venv --clear " + quoteShellArgument(configuration.GraphitiVirtualEnvPath) + " >/dev/null",
		"uv pip install --python " + quoteShellArgument(pythonPath) + " -r " + quoteShellArgument(filepath.Join(configuration.GraphitiPackagePath, "requirements.txt")) + " >/dev/null",
		"printf '%s\\n' '#!/bin/sh' 'PYTHONPATH=/opt/internkim exec " + pythonPath + " -m graphiti_memoryd \"$@\"' > " + quoteShellArgument(blueclaw.GraphitiMemorydPath),
		"chmod 755 " + quoteShellArgument(blueclaw.GraphitiMemorydPath),
		"chown -R root:blueclaw /opt/internkim",
		"chmod -R u=rwX,g=rX,o=rX /opt/internkim",
	}, "\n")
	_, errorValue := service.commandRunner().Run(ctx, ExecutableCommand{
		ExecutableName: "sh",
		Arguments:      []string{"-c", command},
	})
	return errorValue
}

func tenantHostBlueclawRuntimeConfiguration(manifest Manifest, paths RuntimePaths, configuration hostRuntimeConfigurationDocument) (string, error) {
	return blueclaw.BlueclawRuntimeConfigDocumentWithOptions(blueclaw.RuntimeConfigOptions{
		ModelName:                configuration.ModelName,
		BaseURL:                  hostLocalURL(configuration.BlueclawPort),
		CapabilitySocketPath:     tenantCapabilitySocketPath(paths),
		CapabilityVSockPort:      configuration.CapabilityVSockPort,
		GraphitiEndpoint:         blueclaw.GraphitiEndpoint,
		MattermostBaseURL:        manifest.MattermostInstance.InternalURL,
		HostWorkspacePath:        paths.BlueclawWorkspacePath,
		RootFilesystemImagePath:  configuration.RootFilesystemImagePath,
		WorkspaceImagePath:       configuration.WorkspaceImagePath,
		HostHTTPListenAddress:    "127.0.0.1:" + intString(configuration.BlueclawPort),
		HealthPortOrService:      intString(configuration.HealthPort),
		GuestHTTPPortOrService:   intString(configuration.GuestHTTPPort),
		LogDirectoryPath:         filepath.Join(paths.BlueclawRootPath, "logs", "supervisor"),
		RuntimeDirectoryPath:     hostRuntimeDirectoryPath(manifest, configuration),
		OutboundHostDeviceName:   "bctap" + strconv.Itoa(configuration.NetworkIndex),
		OutboundGuestMACAddress:  tenantGuestMACAddress(configuration.NetworkIndex),
		OutboundNetworkCIDR:      tenantNetworkCIDR(configuration.NetworkIndex, "0/30"),
		OutboundHostAddressCIDR:  tenantNetworkCIDR(configuration.NetworkIndex, "1/30"),
		OutboundGuestAddressCIDR: tenantNetworkCIDR(configuration.NetworkIndex, "2/30"),
		OutboundGuestGateway:     tenantNetworkCIDR(configuration.NetworkIndex, "1"),
		BridgeListenAddress:      "127.0.0.1:" + intString(configuration.BridgePort),
	})
}

func writeHostRuntimeSystemdUnits(systemdDirectoryPath string, manifest Manifest, paths RuntimePaths, configuration hostRuntimeConfigurationDocument) error {
	if errorValue := os.MkdirAll(systemdDirectoryPath, 0o755); errorValue != nil {
		return errorValue
	}
	units := map[string]string{
		hostRuntimeAdmindServiceName(manifest):      hostRuntimeAdmindServiceUnit(manifest, paths, configuration),
		hostRuntimeCapabilitydServiceName(manifest): hostRuntimeCapabilitydServiceUnit(manifest, paths, configuration),
		hostRuntimeGraphitiServiceName(manifest):    hostRuntimeGraphitiServiceUnit(paths, configuration),
		hostRuntimeBlueclawServiceName(manifest):    hostRuntimeBlueclawServiceUnit(manifest, paths),
	}
	for serviceName, document := range units {
		if errorValue := os.WriteFile(filepath.Join(systemdDirectoryPath, serviceName), []byte(document), 0o644); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func hostRuntimeAdmindServiceUnit(manifest Manifest, paths RuntimePaths, configuration hostRuntimeConfigurationDocument) string {
	return `[Unit]
Description=InternKim tenant admin gateway ` + manifest.TenantID + `
After=network-online.target internkim-mattermost-` + manifest.TenantID + `.service ` + hostRuntimeBlueclawServiceName(manifest) + `
Wants=network-online.target internkim-mattermost-` + manifest.TenantID + `.service

[Service]
User=root
ExecStart=` + blueclaw.AdmindBinaryPath +
		" --listen 127.0.0.1:" + intString(configuration.AdmindPort) +
		" --mattermost-url " + manifest.MattermostInstance.InternalURL +
		" --mattermost-admin-password " + filepath.Join(paths.InternKimSecretsPath, "mm-admin-pass") +
		" --blueclaw-url " + hostLocalURL(configuration.BlueclawPort) +
		" --state-dir " + filepath.Join(paths.InternKimPath, "state", "admin") +
		" --calendar-db " + filepath.Join(paths.InternKimPath, "state", "admin", "calendar.sqlite") +
		" --calendar-secrets-dir " + filepath.Join(paths.InternKimPath, "secrets", "calendar") +
		" --mail-db " + filepath.Join(paths.InternKimPath, "state", "admin", "mail.sqlite") +
		" --attendance-db " + filepath.Join(paths.InternKimPath, "state", "admin", "attendance.sqlite") +
		" --admin-email-path " + filepath.Join(paths.InternKimPath, "config", "admin-email") +
		" --device-url-path " + filepath.Join(paths.InternKimPath, "env", "device-url") +
		" --fleet-id-path " + filepath.Join(paths.InternKimPath, "env", "fleet-id") +
		" --openrouter-key " + filepath.Join(paths.InternKimSecretsPath, "llm-device-token") +
		" --blueclaw-workspace " + paths.BlueclawWorkspacePath + `
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`
}

func hostRuntimeCapabilitydServiceUnit(manifest Manifest, paths RuntimePaths, configuration hostRuntimeConfigurationDocument) string {
	command := blueclaw.CapabilitydBinaryPath +
		" --socket " + tenantCapabilitySocketPath(paths) +
		" --companion-url " + hostLocalURL(configuration.AdmindPort) + "/_internkim/companion" +
		" --mattermost-url " + manifest.MattermostInstance.InternalURL +
		" --mattermost-token " + filepath.Join(paths.InternKimSecretsPath, "mattermost-bot-token") +
		" --mattermost-interactive-token " + filepath.Join(paths.InternKimPath, "state", "admin", "mattermost-interactive-token") +
		" --blueclaw-url " + hostLocalURL(configuration.BlueclawPort) +
		" --blueclaw-workspace " + paths.BlueclawWorkspacePath +
		" --openrouter-url " + configuration.GatewayURL +
		" --openrouter-web-url " + configuration.GatewayURL +
		" --openrouter-embedding-url " + gatewayEmbeddingURL(configuration.GatewayURL) +
		" --openrouter-key " + filepath.Join(paths.InternKimSecretsPath, "llm-device-token") +
		" --local-inference-mode remote" +
		" --fleet-id-path " + filepath.Join(paths.InternKimPath, "env", "fleet-id")
	gatewaySharedSecretPath := filepath.Join(paths.InternKimSecretsPath, "llm-gateway-shared-secret")
	if strings.TrimSpace(configuration.GatewaySharedSecret) != "" || fileExists(gatewaySharedSecretPath) {
		command += " --openrouter-gateway-secret " + gatewaySharedSecretPath
	}
	return `[Unit]
Description=InternKim tenant capability daemon ` + manifest.TenantID + `
After=network-online.target internkim-mattermost-` + manifest.TenantID + `.service ` + hostRuntimeAdmindServiceName(manifest) + `
Wants=network-online.target ` + hostRuntimeAdmindServiceName(manifest) + `

[Service]
User=root
RuntimeDirectory=internkim-` + manifest.TenantID + `
ExecStart=` + command + `
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`
}

func hostRuntimeGraphitiServiceUnit(paths RuntimePaths, configuration hostRuntimeConfigurationDocument) string {
	return `[Unit]
Description=InternKim tenant Graphiti memory daemon
After=network-online.target
Wants=network-online.target

[Service]
User=root
Environment=HOME=` + paths.BlueclawRootPath + `
Environment=BLUECLAW_GRAPHITI_KUZU_PATH=` + filepath.Join(paths.BlueclawWorkspacePath, ".blueclaw", "graphiti", "kuzu") + `
Environment=BLUECLAW_GRAPHITI_LISTEN_ADDRESS=127.0.0.1
Environment=BLUECLAW_GRAPHITI_PORT=` + intString(configuration.GraphitiPort) + `
ExecStart=` + blueclaw.GraphitiMemorydPath + `
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`
}

func hostRuntimeBlueclawServiceUnit(manifest Manifest, paths RuntimePaths) string {
	return `[Unit]
Description=InternKim tenant Blueclaw ` + manifest.TenantID + `
After=network-online.target ` + hostRuntimeCapabilitydServiceName(manifest) + ` ` + hostRuntimeGraphitiServiceName(manifest) + `
Wants=network-online.target ` + hostRuntimeCapabilitydServiceName(manifest) + ` ` + hostRuntimeGraphitiServiceName(manifest) + `

[Service]
User=root
Environment=RUST_LOG=debug
ExecStart=` + blueclaw.BlueclawSupervisorBinaryPath + ` -runtime ` + filepath.Join(paths.BlueclawRootPath, "config", "runtime.json") + `
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`
}

func hostRuntimeStatus(manifest Manifest, paths RuntimePaths, configuration hostRuntimeConfigurationDocument) HostRuntimeStatus {
	return HostRuntimeStatus{
		TenantID:                  manifest.TenantID,
		MattermostURL:             manifest.MattermostInstance.InternalURL,
		AdmindURL:                 hostLocalURL(configuration.AdmindPort),
		BlueclawURL:               hostLocalURL(configuration.BlueclawPort),
		GraphitiURL:               hostLocalURL(configuration.GraphitiPort),
		ServiceNames:              hostRuntimeServiceNameMap(manifest),
		RuntimeConfigurationPath:  filepath.Join(paths.BlueclawRootPath, "config", "runtime.json"),
		RootFilesystemImagePath:   configuration.RootFilesystemImagePath,
		WorkspaceImagePath:        configuration.WorkspaceImagePath,
		GraphitiVirtualEnvPath:    configuration.GraphitiVirtualEnvPath,
		MattermostBotTokenReady:   fileExists(filepath.Join(paths.InternKimSecretsPath, "mattermost-bot-token")),
		LLMDeviceTokenReady:       fileExists(filepath.Join(paths.InternKimSecretsPath, "llm-device-token")),
		GatewaySharedSecretReady:  fileExists(filepath.Join(paths.InternKimSecretsPath, "llm-gateway-shared-secret")),
		GraphitiReady:             fileExists(filepath.Join(configuration.GraphitiVirtualEnvPath, "bin", "python")),
		RuntimeConfigurationReady: fileExists(filepath.Join(paths.BlueclawRootPath, "config", "runtime.json")),
	}
}

func tenantCapabilitySocketPath(paths RuntimePaths) string {
	return filepath.Join(paths.InternKimPath, "run", "capability.sock")
}

func hostRuntimeDirectoryPath(manifest Manifest, configuration hostRuntimeConfigurationDocument) string {
	return filepath.Join(configuration.RuntimeDirectoryBasePath, manifest.TenantID)
}

func hostRuntimeServiceNameMap(manifest Manifest) map[string]string {
	return map[string]string{
		"admind":      hostRuntimeAdmindServiceName(manifest),
		"capabilityd": hostRuntimeCapabilitydServiceName(manifest),
		"graphiti":    hostRuntimeGraphitiServiceName(manifest),
		"blueclaw":    hostRuntimeBlueclawServiceName(manifest),
	}
}

func hostRuntimeServiceNames(manifest Manifest) []string {
	names := hostRuntimeServiceNameMap(manifest)
	return []string{names["graphiti"], names["capabilityd"], names["blueclaw"], names["admind"]}
}

func hostRuntimeAdmindServiceName(manifest Manifest) string {
	return "internkim-tenant-admind-" + manifest.TenantID + ".service"
}

func hostRuntimeCapabilitydServiceName(manifest Manifest) string {
	return "internkim-tenant-capabilityd-" + manifest.TenantID + ".service"
}

func hostRuntimeGraphitiServiceName(manifest Manifest) string {
	return "internkim-tenant-graphiti-" + manifest.TenantID + ".service"
}

func hostRuntimeBlueclawServiceName(manifest Manifest) string {
	return "internkim-tenant-blueclaw-" + manifest.TenantID + ".service"
}

func hostLocalURL(port int) string {
	return "http://127.0.0.1:" + intString(port)
}

func gatewayEmbeddingURL(gatewayURL string) string {
	trimmedURL := strings.TrimSpace(gatewayURL)
	if strings.HasSuffix(trimmedURL, "/chat/completions") {
		return strings.TrimSuffix(trimmedURL, "/chat/completions") + "/embeddings"
	}
	return strings.TrimRight(trimmedURL, "/") + "/embeddings"
}

func tenantNumericSuffix(tenantID string) int {
	parts := strings.Split(strings.TrimSpace(tenantID), "-")
	if len(parts) == 0 {
		return 1
	}
	value, errorValue := strconv.Atoi(parts[len(parts)-1])
	if errorValue != nil || value <= 0 {
		return 1
	}
	return value
}

func defaultTenantHostRuntimePortBase(tenantIndex int) int {
	return 18100 + (tenantIndex-1)*100
}

func tenantGuestMACAddress(networkIndex int) string {
	return fmt.Sprintf("AA:FC:00:00:%02X:%02X", networkIndex/256, networkIndex%256)
}

func tenantNetworkCIDR(networkIndex int, suffix string) string {
	return "172.31." + strconv.Itoa(networkIndex) + "." + suffix
}

func firstNonEmptyTenantString(values ...string) string {
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			return trimmedValue
		}
	}
	return ""
}

func firstPositiveTenantInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func quoteShellArgument(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
