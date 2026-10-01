package blueclaw

import (
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
	LiteRTModelPath                      = "/root/.internkim/models/gemma-4-E4B-it.litertlm"
	LiteRTModelFilename                  = "gemma-4-E4B-it.litertlm"
	BlueclawDefaultModelName             = modelladder.PrimaryModel
	BlueclawDeliveryConfigPath           = "/var/lib/blueclaw/delivery/config"
	BlueclawDeliveryRuntimePath          = "/var/lib/blueclaw/delivery/runtime/current"
	BlueclawRuntimeInstanceDirectoryPath = "/var/lib/bc"
	BlueclawDeliverySkillsPath           = "/var/lib/blueclaw/delivery/skills"
	BlueclawMessengerPlatform            = "buzz"
	BuzzRelayName                        = "buzz-relay"
	BuzzAdminName                        = "buzz-admin"
	BuzzRelayServiceName                 = "buzz-relay"
	BuzzAdminBinaryPath                  = "/usr/local/bin/buzz-admin"
	BuzzRelayDatabaseName                = "buzz"
	BuzzRelayDatabaseUser                = "buzz"
	BuzzRelayKeyEnvironmentFilePath      = "/root/.internkim/secrets/buzz-relay-env"
	BuzzRelayDatabaseEnvironmentFilePath = "/root/.internkim/secrets/buzz-relay-db"
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
	BuzzRelayCertificatePath       = "/root/.internkim/tls/relay.crt"
	BuzzRelayPublicURLFilePath     = "/root/.internkim/env/buzz-relay-public-url"
	BuzzRelayRedisURL              = "redis://127.0.0.1:6379"
	BuzzRelayArtifactPath          = ".dependency/buzz-relay"
	BuzzRelayAmd64ArtifactPath     = ".dependency/buzz-relay-linux-amd64"
	BuzzMediaServiceName           = "buzz-media"
	BuzzMediaProgramName           = "versitygw"
	BuzzMediaVersion               = "1.8.0"
	BuzzMediaRootPath              = "/var/lib/buzz-media"
	BuzzMediaAddress               = "127.0.0.1:9000"
	BuzzMediaHealthPath            = "/_health"
	BuzzMediaBucket                = "buzz-media"
	BuzzRelayImportOverrideEnvPath = "/root/.internkim/secrets/buzz-relay-import-override"
	BuzzMigrateName                = "buzz-migrate"
	BuzzMigrateBinaryPath          = "/usr/local/bin/buzz-migrate"
	BuzzMigrateMarkerPath          = "/root/.internkim/state/buzz-migrated"
	BuzzAccountLinksPath           = "/root/.internkim/state/admin/buzz-account-links.json"
	ChatdName                      = "chatd"
	ChatdServiceName               = "chatd"
	ChatdDropInDirectory           = "/etc/systemd/system/chatd.service.d"
	ChatdListenPort                = "18090"
	// The agent runs in the guest and reaches this machine on the outbound tap,
	// so chatd answers there. The address is this machine's own interface, which
	// the network beyond it cannot route to.
	ChatdListenHostname      = "172.31.0.1"
	ChatdEndpoint            = "http://172.31.0.1:18090"
	ChatdHealthPath          = "/healthz"
	RelayName                = "internkim-relay"
	RelayServiceName         = "internkim-relay"
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

	RelayAgentKeyPath        = "/etc/internkim/agent-key"
	RelayUserName            = "internkim"
	MaildName                = "internkim-maild"
	RenderCompanyRuntimeName = "render-company-runtime"
	DeviceBrowserName        = "moli"
	AgentBrowserName         = "agent-browser"
	BunProgramName           = "bun"
	PackageResolverName      = "uv"
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

func BlueclawHealthCheckURL() string {
	return BlueclawBaseURL + BlueclawHealthCheckPath
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

func BuzzRelayReadinessURL() string {
	return "http://" + BuzzRelayBindAddress + BuzzRelayReadinessPath
}

func ChatdLegacyTLSDropInPaths() []string {
	return []string{ChatdDropInDirectory + "/tls.conf", ChatdDropInDirectory + "/tls-debug.conf"}
}
