package blueclaw

import (
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/modelladder"
)

const (
	BlueclawName                         = "blueclaw"
	BlueclawServiceName                  = "blueclaw"
	CapabilitydName                      = "internkim-capabilityd"
	AdmindName                           = "internkim-admind"
	CapabilitydServiceName               = "internkim-capabilityd"
	AdmindServiceName                    = "internkim-admind"
	BlueclawUser                         = "blueclaw"
	BlueclawHomePath                     = "/home/blueclaw"
	BlueclawWorkspacePath                = "/root/.blueclaw/workspace"
	BlueclawRuntimeConfigPath            = "/root/.blueclaw/config/runtime.json"
	BlueclawPolicyConfigPath             = "/root/.blueclaw/config/policy.json"
	BlueclawPOSIXHelperPath              = "/usr/local/bin/blueclaw-posix-helper"
	CapabilitydBinaryPath                = "/usr/local/bin/internkim-capabilityd"
	AdmindBinaryPath                     = "/usr/local/bin/internkim-admind"
	OpenRouterKeyPath                    = "/root/.internkim/secrets/openrouter-api-key"
	BlueclawBaseURL                      = "http://127.0.0.1:8080"
	AdmindBaseURL                        = "http://127.0.0.1:18080"
	BlueclawDatabaseName                 = "blueclaw"
	BlueclawHealthCheckPath              = "/admin/api/health"
	BlueclawSubmodulePath                = ".dependency/blueclaw"
	BlueclawPolicyAdminID                = "00000000-0000-0000-0000-000000000001"
	CapabilitySocketPath                 = "/run/internkim/capability.sock"
	AdmindSocketPath                     = "/run/internkim/admind.sock"
	BlueclawMattermostTokenPath          = "/root/.internkim/secrets/mattermost-bot-token"
	BlueclawMattermostLocalURL           = "http://127.0.0.1:8065"
	BlueclawDefaultModelName             = modelladder.PrimaryModel
	BlueclawDefaultModelContextTokens    = 1048576
	BlueclawDeliveryConfigPath           = "/var/lib/blueclaw/delivery/config"
	BlueclawDeliveryRuntimePath          = "/var/lib/blueclaw/delivery/runtime/current"
	BlueclawAdminAssertionKeyName        = "admin-assertion-key"
	BlueclawRuntimeInstanceDirectoryPath = "/var/lib/bc"
	BlueclawMessengerPlatform            = "buzz"
	BuzzRelayName                        = "buzz-relay"
	BuzzAdminName                        = "buzz-admin"
	BuzzRelayServiceName                 = "buzz-relay"
	BuzzRelayDatabaseName                = "buzz"
	BuzzRelayDatabaseUser                = "buzz"
	BuzzRelayBindAddress                 = "127.0.0.1:3000"

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
	BuzzRelayRedisURL              = "redis://127.0.0.1:6379"
	BuzzRelayArtifactPath          = ".dependency/buzz-relay"
	BuzzRelayAmd64ArtifactPath     = ".dependency/buzz-relay-linux-amd64"
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
	ChatdDropInDirectory           = "/etc/systemd/system/chatd.service.d"
	ChatdStateDirectoryPath        = "/root/.internkim/state/chatd"
	ChatdBinaryPath                = "/usr/local/bin/chatd"
	ChatdEnvironmentFilePath       = "/root/.internkim/secrets/chatd-env"
	ChatdListenPort                = "18090"
	ChatdHealthPath                = "/healthz"
	ChatdBotUserName               = "internkim"
	RelayName                      = "internkim-relay"
	RelayServiceName               = "internkim-relay"
	RelayServicePath               = "/etc/systemd/system/internkim-relay.service"
	RelayBinaryPath                = "/usr/local/bin/internkim-relay"
	RelayEnvironmentFilePath       = "/etc/internkim/relay.env"

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
	BunProgramName             = "bun"
	PackageResolverName        = "uv"
	BuzzPremigrateSnapshotPath = "/root/.internkim/state/buzz-premigrate.sql"
)

// Who may connect to a socket in the run directory is said by its mode, its
// owner and its group; the run directory itself only lets accounts pass through.
const (
	CapabilitySocketMode = 0o660
	AdmindSocketMode     = 0o660
	// BlueclawACPSocketMode is what blueclaw's internal/acpsession chmods its
	// socket to, held there by TestTheSocketLetsTheGroupItsDirectoryGivesItConnect.
	// It is written down here because that package cannot be imported.
	BlueclawACPSocketMode = 0o660
)

var secretsTheBlueclawUserMustNotRead = []string{
	"/root/.internkim/secrets/openrouter-api-key",
	BlueclawMattermostTokenPath,
	"/root/.internkim/secrets/device-secret",
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

func BlueclawSubmoduleRoot(scriptDir string) string {
	return filepath.Join(scriptDir, BlueclawSubmodulePath)
}

// BuzzRelayArtifactPathFor is where tools/prepare-buzz-relay leaves the messenger
// built for one Debian architecture. arm64 is the appliance's directory and keeps
// its name; amd64 sits beside it.
func BuzzRelayArtifactPathFor(debianArchitecture string) string {
	if debianArchitecture == "amd64" {
		return BuzzRelayAmd64ArtifactPath
	}
	return BuzzRelayArtifactPath
}
