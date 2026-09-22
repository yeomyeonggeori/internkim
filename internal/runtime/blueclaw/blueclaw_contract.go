package blueclaw

import (
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/internal/modelladder"
)

const (
	retiredLLMDServiceName             = "blueclaw-llmd"
	retiredLLMDServicePath             = "/etc/systemd/system/blueclaw-llmd.service"
	retiredLLMDBinaryPath              = "/usr/local/bin/blueclaw-llmd"
	retiredLLMDCredentialDirectoryPath = "/var/lib/internkim/llmd-credentials"

	BlueclawName                          = "blueclaw"
	BlueclawSupervisorName                = "blueclaw-supervisor"
	BlueclawServiceName                   = "blueclaw"
	CapabilitydName                       = "internkim-capabilityd"
	AdmindName                            = "internkim-admind"
	LocalLLMRunnerName                    = "internkim-local-llm-runner"
	CapabilitydServiceName                = "internkim-capabilityd"
	AdmindServiceName                     = "internkim-admind"
	BlueclawUser                          = "blueclaw"
	BlueclawHomePath                      = "/home/blueclaw"
	BlueclawRootPath                      = "/root/.blueclaw"
	BlueclawWorkspacePath                 = "/root/.blueclaw/workspace"
	BlueclawConfigPath                    = "/root/.blueclaw/config"
	BlueclawMigrationPath                 = "/root/.blueclaw/workspace/.blueclaw/runtime/current/migrations"
	BlueclawRuntimeConfigPath             = "/root/.blueclaw/config/runtime.json"
	BlueclawPolicyConfigPath              = "/root/.blueclaw/config/policy.json"
	BlueclawServicePath                   = "/etc/systemd/system/blueclaw.service"
	CapabilitydServicePath                = "/etc/systemd/system/internkim-capabilityd.service"
	AdmindServicePath                     = "/etc/systemd/system/internkim-admind.service"
	BlueclawBinaryPath                    = "/usr/local/bin/blueclaw"
	BlueclawSupervisorBinaryPath          = "/usr/local/bin/blueclaw-supervisor"
	BlueclawPOSIXHelperPath               = "/usr/local/bin/blueclaw-posix-helper"
	CapabilitydBinaryPath                 = "/usr/local/bin/internkim-capabilityd"
	AdmindBinaryPath                      = "/usr/local/bin/internkim-admind"
	LocalLLMRunnerBinaryPath              = "/usr/local/bin/internkim-local-llm-runner"
	OpenRouterKeyPath                     = "/root/.internkim/secrets/openrouter-api-key"
	BlueclawBaseURL                       = "http://127.0.0.1:8080"
	AdmindBaseURL                         = "http://127.0.0.1:18080"
	BlueclawDatabaseName                  = "blueclaw"
	BlueclawGuestWorkspacePath            = "/workspace"
	BlueclawGuestConfigPath               = "/workspace/.blueclaw/config"
	BlueclawGuestMigrationPath            = "/workspace/.blueclaw/runtime/current/migrations"
	BlueclawGuestRuntimeConfigPath        = "/workspace/.blueclaw/config/runtime.json"
	BlueclawGuestPolicyConfigPath         = "/workspace/.blueclaw/config/policy.json"
	BlueclawGuestDatabaseSocketPath       = "/workspace/.blueclaw/postgres"
	BlueclawGuestDatabaseConnectionString = "user=blueclaw dbname=blueclaw host=/workspace/.blueclaw/postgres sslmode=disable"
	BlueclawHealthCheckPath               = "/admin/api/health"
	BlueclawSubmodulePath                 = ".dependency/blueclaw"
	BlueclawPolicyAdminID                 = "00000000-0000-0000-0000-000000000001"
	BlueclawRuntimeLogLevel               = "debug"
	BlueclawSessionDirectory              = "/root/.blueclaw/workspace/sessions"
	CapabilitySocketPath                  = "/run/internkim/capability.sock"
	AdmindSocketPath                      = "/run/internkim/admind.sock"
	CapabilityVSockHostCID                = 2
	CapabilityVSockPort                   = 7000
	BlueclawMattermostTokenPath           = "/root/.internkim/secrets/mattermost-bot-token"
	BlueclawMattermostLocalURL            = "http://127.0.0.1:8065"
	LiteRTModelPath                       = "/root/.internkim/models/gemma-4-E4B-it.litertlm"
	LiteRTModelSourceURL                  = "https://huggingface.co/litert-community/gemma-4-E4B-it-litert-lm/resolve/main/gemma-4-E4B-it.litertlm"
	LiteRTModelRepository                 = "litert-community/gemma-4-E4B-it-litert-lm"
	LiteRTModelFilename                   = "gemma-4-E4B-it.litertlm"
	BlueclawDefaultModelName              = modelladder.PrimaryModel
	BlueclawDefaultModelContextTokens     = 1048576
	BlueclawCloudHypervisorPath           = "/usr/local/bin/cloud-hypervisor"
	BlueclawVirtiofsdPath                 = "/usr/local/bin/virtiofsd"
	BlueclawVfkitPath                     = "/usr/local/bin/vfkit"
	BlueclawDeliveryPath                  = "/var/lib/blueclaw/delivery"
	BlueclawDeliveryReadOnlyPath          = "/var/lib/blueclaw/delivery-ro"
	BlueclawDeliveryConfigPath            = "/var/lib/blueclaw/delivery/config"
	BlueclawDeliverySecretsPath           = "/var/lib/blueclaw/delivery/secrets"
	BlueclawDeliveryRuntimePath           = "/var/lib/blueclaw/delivery/runtime/current"
	BlueclawGuestDeliveryRuntimePath      = "/delivery/runtime/current"
	BlueclawGuestDeliverySecretsPath      = "/delivery/secrets"
	BlueclawAdminAssertionKeyName         = "admin-assertion-key"
	BlueclawDeliverySkillsPath            = "/var/lib/blueclaw/delivery/skills"
	BlueclawGuestDeliverySkillsPath       = "/delivery/skills"
	BlueclawDeliveryServiceName           = "internkim-blueclaw-delivery.service"
	BlueclawDeliveryServicePath           = "/etc/systemd/system/internkim-blueclaw-delivery.service"
	CloudHypervisorMonitorName            = "cloudHypervisor"
	VfkitMonitorName                      = "vfkit"
	BlueclawVirtualMachineMonitor         = CloudHypervisorMonitorName
	BlueclawRuntimeInstanceDirectoryPath  = "/var/lib/bc"
	BlueclawRuntimeInstallPath            = "/opt/internkim/blueclaw-runtime"
	BlueclawRuntimeManifestPath           = "/opt/internkim/blueclaw-runtime/manifest.json"
	BlueclawPayloadManifestPath           = "/opt/internkim/blueclaw-runtime/payload-manifest.json"
	BlueclawKernelImagePath               = "/opt/internkim/blueclaw-runtime/vmlinux.bin"
	BlueclawRootFilesystemImagePath       = "/opt/internkim/blueclaw-runtime/rootfs.ext4"
	BlueclawWorkspaceImagePath            = "/var/lib/blueclaw/workspace.ext4"
	BlueclawRuntimeArtifactPath           = ".dependency/blueclaw-runtime"
	BlueclawPayloadArtifactPath           = ".dependency/blueclaw-payload"
	BlueclawGuestRuntimeCurrentPath       = "/workspace/.blueclaw/runtime/current"
	BlueclawGuestBinaryPath               = "/workspace/.blueclaw/runtime/current/bin/blueclaw"
	BlueclawSupervisorLogDirectoryPath    = "/var/log/blueclaw-supervisor"
	BlueclawBridgeAuthorizedKeysPath      = "/var/lib/blueclaw/authorized_companions"
	BlueclawBridgeListenAddress           = "127.0.0.1:7778"
	BlueclawMessengerPlatform             = "buzz"
	BuzzRelayName                         = "buzz-relay"
	BuzzAdminName                         = "buzz-admin"
	BuzzRelayServiceName                  = "buzz-relay"
	BuzzRelayServicePath                  = "/etc/systemd/system/buzz-relay.service"
	BuzzRelayBinaryPath                   = "/usr/local/bin/buzz-relay"
	BuzzAdminBinaryPath                   = "/usr/local/bin/buzz-admin"
	BuzzRelayDatabaseName                 = "buzz"
	BuzzRelayDatabaseUser                 = "buzz"
	BuzzRelayDatabasePasswordPath         = "/root/.internkim/secrets/buzz-db-pass"
	BuzzRelayKeyEnvironmentFilePath       = "/root/.internkim/secrets/buzz-relay-env"
	BuzzRelayDatabaseEnvironmentFilePath  = "/root/.internkim/secrets/buzz-relay-db"
	BuzzRelayBindAddress                  = "127.0.0.1:3000"

	// The relay answers /_readiness twice: on its own bind address, and again on
	// the separate health-only router BUZZ_HEALTH_PORT names, which also serves
	// /_liveness and /_status and binds every interface. Everything that asks
	// whether the messenger is ready asks the bind address, which is loopback
	// and is the same number on the device and on the packaged host. The
	// compose file the package replaces polled the health router on 8081, and
	// that is the only reason a second number was ever written down.
	BuzzRelayHealthPort            = "3001"
	BuzzRelayReadinessPath         = "/_readiness"
	BuzzRelayLocalURL              = "ws://127.0.0.1:3000"
	BuzzRelayCertificatePath       = "/root/.internkim/tls/relay.crt"
	BuzzRelayPublicURLFilePath     = "/root/.internkim/env/buzz-relay-public-url"
	DeviceURLFilePath              = "/root/.internkim/env/device-url"
	BuzzRelayRedisURL              = "redis://127.0.0.1:6379"
	BuzzRelayArtifactPath          = ".dependency/buzz-relay"
	BuzzMediaServiceName           = "buzz-media"
	BuzzMediaServicePath           = "/etc/systemd/system/buzz-media.service"
	BuzzMediaProgramName           = "versitygw"
	BuzzMediaBinaryPath            = "/usr/local/bin/versitygw"
	BuzzMediaVersion               = "1.8.0"
	BuzzMediaRootPath              = "/var/lib/buzz-media"
	BuzzMediaAddress               = "127.0.0.1:9000"
	BuzzMediaHealthPath            = "/_health"
	BuzzMediaEnvironmentFilePath   = "/root/.internkim/secrets/buzz-media-env"
	BuzzMediaPasswordPath          = "/root/.internkim/secrets/buzz-media-pass"
	BuzzMediaBucket                = "buzz-media"
	BuzzMediaAccessKey             = "buzzrelay"
	BuzzRelayS3EnvironmentFilePath = "/root/.internkim/secrets/buzz-relay-s3"
	BuzzRelayImportOverrideEnvPath = "/root/.internkim/secrets/buzz-relay-import-override"
	BuzzMigrateName                = "buzz-migrate"
	BuzzMigrateBinaryPath          = "/usr/local/bin/buzz-migrate"
	BuzzMigrateMarkerPath          = "/root/.internkim/state/buzz-migrated"
	BuzzAccountLinksPath           = "/root/.internkim/state/admin/buzz-account-links.json"
	ChatdName                      = "chatd"
	ChatdServiceName               = "chatd"
	ChatdServicePath               = "/etc/systemd/system/chatd.service"
	ChatdBinaryPath                = "/usr/local/bin/chatd"
	ChatdEnvironmentFilePath       = "/root/.internkim/secrets/chatd-env"
	ChatdListenPort                = "18090"
	// The agent runs in the guest and reaches this machine on the outbound tap,
	// so chatd answers there. The address is this machine's own interface, which
	// the network beyond it cannot route to.
	ChatdListenHostname      = "172.31.0.1"
	ChatdEndpoint            = "http://172.31.0.1:18090"
	ChatdHealthPath          = "/healthz"
	ChatdBotUserName         = "internkim"
	RelayName                = "internkim-relay"
	RelayServiceName         = "internkim-relay"
	RelayServicePath         = "/etc/systemd/system/internkim-relay.service"
	RelayBinaryPath          = "/usr/local/bin/internkim-relay"
	RelayEnvironmentFilePath = "/etc/internkim/relay.env"

	// Where the relay keeps its own state, and the one place the device and the
	// packaged host differ. The device's state root is 0755 and the relay's
	// directory has been inside it since before the package existed. The
	// package's state root is 0700 root:root because a company's identity sits
	// under it, and the relay runs as an unprivileged account that could not
	// then traverse into its own directory, so on that host its state is a
	// sibling of the company tree instead of a child.
	RelayStateDirectoryName            = "internkim/relay"
	CompanyHostRelayStateDirectoryName = "internkim-relay"

	RelayAgentKeyPath          = "/etc/internkim/agent-key"
	RelayUserName              = "internkim"
	MaildName                  = "internkim-maild"
	RenderCompanyRuntimeName   = "render-company-runtime"
	DeviceBrowserName          = "moli"
	AgentBrowserName           = "agent-browser"
	BuzzPremigrateSnapshotPath = "/root/.internkim/state/buzz-premigrate.sql"
)

var secretsTheBlueclawUserMustNotRead = []string{
	"/root/.internkim/secrets/openrouter-api-key",
	BlueclawMattermostTokenPath,
	"/root/.internkim/secrets/device-secret",
	LiteRTModelPath,
}

func SecretIsolationShellTest() string {
	readabilityTests := make([]string, 0, len(secretsTheBlueclawUserMustNotRead))
	for _, path := range secretsTheBlueclawUserMustNotRead {
		readabilityTests = append(readabilityTests, "test -r "+path)
	}
	return strings.Join(readabilityTests, " || ")
}

func BlueclawHealthCheckURL() string {
	return BlueclawBaseURL + BlueclawHealthCheckPath
}

func BlueclawHealthCheckCommand() string {
	return "curl --max-time 15 -fsS " + BlueclawHealthCheckURL() + " >/dev/null && echo ok || echo no"
}

const capabilitydHealthReportFilter = `if .status != "ok" then "no" else ([.providers // {} | to_entries[] | select(.value.configured and (.value.available | not)) | .key] | sort | if length == 0 then "ok" else "unready:" + join(",") end) end`

func CapabilitydHealthCheckCommand() string {
	return `capabilityd_health="$(curl --max-time 5 -fsS --unix-socket ` + CapabilitySocketPath +
		` http://internkim/health 2>/dev/null | jq -r '` + capabilitydHealthReportFilter + `' 2>/dev/null)"; echo "${capabilityd_health:-no}"`
}

func RetireLLMDLeftByEarlierReleasesCommand() string {
	return strings.Join([]string{
		"systemctl disable --now " + retiredLLMDServiceName + " >/dev/null 2>&1 || true",
		"rm -f " + retiredLLMDServicePath,
		"rm -rf " + retiredLLMDCredentialDirectoryPath,
		"rm -f " + retiredLLMDBinaryPath,
		"systemctl daemon-reload",
	}, "\n")
}

func RetiredLLMDServiceIsGoneCommand() string {
	return "systemctl is-active " + retiredLLMDServiceName + " 2>/dev/null || true"
}

func BlueclawWorkspaceSkillPath(skillName string) string {
	return filepath.Join(BlueclawWorkspacePath, "skills", skillName)
}

func BlueclawWorkspaceBinaryPath(binaryName string) string {
	return filepath.Join(BlueclawWorkspacePath, "bin", binaryName)
}

func BlueclawSubmoduleRoot(scriptDir string) string {
	return filepath.Join(scriptDir, BlueclawSubmodulePath)
}

// The guest runs its payload from the share and reads its migrations there, so the tree
// the host writes has to be mirrored where virtiofsd serves it. Ownership is the host's
// and passes through untranslated, so root owns it and the mode carries the access: the
// binary has to be executable by the guest's blueclaw, which is not root.
//
// It is appended to a sync command, so the caller runs the pair under set -e; without it
// the pair reports this command's status and a failed sync would read as success.
func deliverySourceRsyncCommand(sourcePath string, deliveredPath string) string {
	return "if [ -d " + sourcePath + " ]; then rsync -a --delete " + sourcePath + "/ " + deliveredPath + "/; fi\n"
}

func BlueclawDeliveryRefreshCommand() string {
	temporaryKeyPath := BlueclawDeliverySecretsPath + "/." + BlueclawAdminAssertionKeyName + ".$$"
	return "\n" +
		"mkdir -p " + BlueclawDeliveryRuntimePath + " " + BlueclawDeliverySkillsPath + " " + BlueclawDeliveryConfigPath + "\n" +
		deliverySourceRsyncCommand(BlueclawWorkspacePath+"/.blueclaw/runtime/current", BlueclawDeliveryRuntimePath) +
		deliverySourceRsyncCommand(BlueclawWorkspacePath+"/skills", BlueclawDeliverySkillsPath) +
		"chown -R root:root " + BlueclawDeliveryConfigPath + " " + BlueclawDeliveryRuntimePath + " " + BlueclawDeliverySkillsPath + "\n" +
		"find " + BlueclawDeliveryConfigPath + " " + BlueclawDeliveryRuntimePath + " " + BlueclawDeliverySkillsPath + " -type d -exec chmod 0755 {} +\n" +
		"find " + BlueclawDeliveryConfigPath + " " + BlueclawDeliveryRuntimePath + " " + BlueclawDeliverySkillsPath + " -type f -exec chmod 0644 {} +\n" +
		"install -d -o 998 -g 971 -m 0700 " + BlueclawDeliverySecretsPath + "\n" +
		"if [ -s " + InternKimCentralPlaneAgentKeyPath + " ]; then rm -f " + temporaryKeyPath + "; if ! install -o 998 -g 971 -m 0400 " + InternKimCentralPlaneAgentKeyPath + " " + temporaryKeyPath + "; then rm -f " + temporaryKeyPath + "; exit 1; fi; if ! mv -f " + temporaryKeyPath + " " + BlueclawDeliverySecretsPath + "/" + BlueclawAdminAssertionKeyName + "; then rm -f " + temporaryKeyPath + "; exit 1; fi; else rm -f " + BlueclawDeliverySecretsPath + "/" + BlueclawAdminAssertionKeyName + "; fi\n" +
		"chown 998:971 " + BlueclawDeliverySecretsPath + "\n" +
		"chmod 0700 " + BlueclawDeliverySecretsPath + "\n" +
		"find " + BlueclawDeliveryRuntimePath + "/bin -type f -exec chmod 0755 {} + 2>/dev/null || true"
}
