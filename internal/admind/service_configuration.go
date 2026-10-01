package admind

import (
	"path/filepath"

	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
	"github.com/yeomyeonggeori/internkim/internal/fleetdomain"
	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type Configuration struct {
	ListenAddress                  string
	ListenSocketPath               string
	MattermostBaseURL              string
	ChatdEndpoint                  string
	ChatdPlatform                  string
	MattermostTeamName             string
	BotUsername                    string
	TaskPublicURL                  string
	APIBaseURL                     string
	BlueclawBaseURL                string
	BlueclawPolicyDeliveryPath     string
	CapabilitySocketPath           string
	StateDirectory                 string
	UsersSyncStatePath             string
	DatabasePath                   string
	TaskDatabasePath               string
	CalendarDatabasePath           string
	MailDatabasePath               string
	AttendanceDatabasePath         string
	CentralPlaneAppURL             string
	CentralPlaneAgentKeyPath       string
	BlueclawAssertionKeyPath       string
	CentralPlaneAppURLPath         string
	CentralPlaneProjectURL         string
	CentralPlaneProjectURLPath     string
	CentralPlanePublishableKey     string
	CentralPlanePublishableKeyPath string
	BridgeMapDatabasePath          string
	MattermostAdminPasswordPath    string
	MattermostTokenPath            string
	OpenRouterKeyPath              string
	OpenRouterModelsURL            string
	ReleaseRegistryURL             string
	ReleaseDownloadTokenPath       string
	MattermostBotTokenPath         string
	AdminEmailPath                 string
	ClaimedAdminEmailPath          string
	APIURLPath                     string
	FleetIDPath                    string
	DeviceURLPath                  string
	TaskPublicURLPath              string
	FleetSecretPath                string
	AdminUIPath                    string
	RepositoryRoot                 string
	SitesRoot                      string
	SiteScaffoldPath               string
	FontsDirectory                 string
	SiteSecretDirectory            string
	SiteSystemdDirectory           string
	IdentityDocumentPath           string
	SoulDocumentPath               string
	BotProfileImagePath            string
	BlueclawWorkspacePath          string
	BlueclawRuntimeConfigPath      string
	BuzzInviteKeyPath              string
	BuzzCommunityID                string
	BuzzRelayURL                   string
	BuzzRelayPublicURL             string
	BuzzRelayPublicURLPath         string
	BuzzLandingBaseURL             string
	BuzzAdminCommandPath           string
	BuzzDatabaseURL                string
	BuzzAccountLinksPath           string
	BuzzKeySeedPath                string
	BuzzRelayKeyPath               string
	CloudflareAccessTeamDomain     string
	CloudflareAccessAUDs           string
	TrustProxyForwardedEmail       bool
	TaskRunNotifyEnabled           bool
	AttendanceNotifyEnabled        bool
	MailNotifyEnabled              bool
	UsersSyncInstallEnabled        bool
}

func DefaultConfiguration() Configuration {
	return Configuration{
		ListenAddress:                  "127.0.0.1:18080",
		ListenSocketPath:               blueclawruntime.AdmindSocketPath,
		TaskRunNotifyEnabled:           true,
		AttendanceNotifyEnabled:        true,
		MailNotifyEnabled:              true,
		UsersSyncInstallEnabled:        true,
		MattermostBaseURL:              "http://127.0.0.1:8065",
		MattermostTeamName:             "internkim",
		BotUsername:                    "internkim",
		BlueclawBaseURL:                "http://127.0.0.1:8080",
		BlueclawPolicyDeliveryPath:     filepath.Join(blueclaw.BlueclawDeliveryConfigPath, "policy.json"),
		CapabilitySocketPath:           blueclawruntime.CapabilitySocketPath,
		StateDirectory:                 "/root/.internkim/state/admin",
		UsersSyncStatePath:             blueclawruntime.InternKimUsersSyncStatePath,
		TaskDatabasePath:               "/root/.internkim/state/flow.sqlite",
		CalendarDatabasePath:           "/root/.internkim/state/calendar.sqlite",
		MailDatabasePath:               "/root/.internkim/state/mail.sqlite",
		AttendanceDatabasePath:         "/root/.internkim/state/attendance.sqlite",
		BridgeMapDatabasePath:          "/root/.internkim/state/bridge-map.sqlite",
		BuzzKeySeedPath:                "/root/.internkim/secrets/buzz-key-seed",
		MattermostAdminPasswordPath:    "/root/.internkim/secrets/mm-admin-pass",
		MattermostTokenPath:            "/root/.internkim/secrets/mattermost-bot-token",
		OpenRouterKeyPath:              "/root/.internkim/secrets/openrouter-api-key",
		OpenRouterModelsURL:            "https://openrouter.ai/api/v1/models",
		ReleaseDownloadTokenPath:       "/root/.internkim/secrets/release-download-token",
		MattermostBotTokenPath:         "/root/.internkim/secrets/mattermost-bot-token",
		AdminEmailPath:                 "/root/.internkim/config/admin-email",
		ClaimedAdminEmailPath:          "/root/.internkim/state/admin/claimed-admin-email",
		APIURLPath:                     "/root/.internkim/env/api-url",
		CentralPlaneAgentKeyPath:       "/root/.internkim/secrets/central-plane-agent-key",
		BlueclawAssertionKeyPath:       "/root/.internkim/secrets/central-plane-agent-key",
		CentralPlaneAppURLPath:         "/root/.internkim/env/central-plane-app-url",
		CentralPlaneProjectURL:         centralplane.DefaultProjectURL,
		CentralPlaneProjectURLPath:     "/root/.internkim/env/central-plane-project-url",
		CentralPlanePublishableKey:     centralplane.DefaultPublishableKey,
		CentralPlanePublishableKeyPath: "/root/.internkim/env/central-plane-publishable-key",
		FleetIDPath:                    "/root/.internkim/env/fleet-id",
		DeviceURLPath:                  "/root/.internkim/env/device-url",
		BuzzRelayPublicURLPath:         blueclawruntime.BuzzRelayPublicURLFilePath,
		TaskPublicURLPath:              "/root/.internkim/env/flow-public-url",
		FleetSecretPath:                "/root/.internkim/secrets/fleet-secret",
		AdminUIPath:                    "/opt/internkim/admin-ui",
		RepositoryRoot:                 "/",
		SitesRoot:                      "/root/.internkim/sites",
		SiteScaffoldPath:               filepath.Join(blueclaw.BlueclawDeliverySkillsPath, "website", "assets", "scaffold", "app"),
		FontsDirectory:                 "/opt/internkim/fonts",
		SiteSecretDirectory:            "/root/.internkim/secrets/sites",
		SiteSystemdDirectory:           "/etc/systemd/system",
		IdentityDocumentPath:           "/root/.internkim/config/identity.json",
		SoulDocumentPath:               "/root/.internkim/config/soul.json",
		BotProfileImagePath:            "/opt/internkim/board-ui/logo.png",
		BlueclawWorkspacePath:          "/root/.blueclaw/workspace",
		BlueclawRuntimeConfigPath:      "/root/.blueclaw/config/runtime.json",
	}
}

// Everyone signs in at the zone itself, and the API address answers only by
// sending them there. A device that does not know its own address claims none.
func companyAddressFrom(apiBaseURL string) string {
	if apiBaseURL == "" {
		return ""
	}
	zone := fleetdomain.Zone(apiBaseURL)
	if zone == "" {
		return apiBaseURL
	}
	return "https://" + zone
}

func (configuration Configuration) withDefaults() Configuration {
	defaultConfiguration := DefaultConfiguration()
	configuration = configuration.withEndpointDefaults(defaultConfiguration)
	configuration = configuration.withDatabaseDefaults(defaultConfiguration)
	configuration = configuration.withCredentialDefaults(defaultConfiguration)
	configuration = configuration.withCentralPlaneDefaults(defaultConfiguration)
	configuration = configuration.withFleetDefaults(defaultConfiguration)
	configuration = configuration.withSiteDefaults(defaultConfiguration)
	configuration = configuration.withPersonaDefaults(defaultConfiguration)
	return configuration
}

func (configuration Configuration) withEndpointDefaults(defaultConfiguration Configuration) Configuration {
	if configuration.ListenAddress == "" {
		configuration.ListenAddress = defaultConfiguration.ListenAddress
	}
	if configuration.ListenSocketPath == "" {
		configuration.ListenSocketPath = defaultConfiguration.ListenSocketPath
	}
	if configuration.MattermostBaseURL == "" {
		configuration.MattermostBaseURL = defaultConfiguration.MattermostBaseURL
	}
	if configuration.APIBaseURL == "" {
		configuration.APIBaseURL = defaultConfiguration.APIBaseURL
	}
	if configuration.BlueclawBaseURL == "" {
		configuration.BlueclawBaseURL = defaultConfiguration.BlueclawBaseURL
	}
	if configuration.BlueclawPolicyDeliveryPath == "" {
		configuration.BlueclawPolicyDeliveryPath = defaultConfiguration.BlueclawPolicyDeliveryPath
	}
	if configuration.CapabilitySocketPath == "" {
		configuration.CapabilitySocketPath = defaultConfiguration.CapabilitySocketPath
	}
	if configuration.StateDirectory == "" {
		configuration.StateDirectory = defaultConfiguration.StateDirectory
	}
	if configuration.UsersSyncStatePath == "" {
		configuration.UsersSyncStatePath = defaultConfiguration.UsersSyncStatePath
	}
	return configuration
}

func (configuration Configuration) withDatabaseDefaults(defaultConfiguration Configuration) Configuration {
	configuration.DatabasePath = resolvedStateDatabasePath(configuration, defaultConfiguration)
	configuration.TaskDatabasePath = stateFilePath(configuration, defaultConfiguration, configuration.TaskDatabasePath, defaultConfiguration.TaskDatabasePath, "flow.sqlite")
	configuration.CalendarDatabasePath = stateFilePath(configuration, defaultConfiguration, configuration.CalendarDatabasePath, defaultConfiguration.CalendarDatabasePath, "calendar.sqlite")
	configuration.MailDatabasePath = stateFilePath(configuration, defaultConfiguration, configuration.MailDatabasePath, defaultConfiguration.MailDatabasePath, "mail.sqlite")
	configuration.AttendanceDatabasePath = stateFilePath(configuration, defaultConfiguration, configuration.AttendanceDatabasePath, defaultConfiguration.AttendanceDatabasePath, "attendance.sqlite")
	configuration.BridgeMapDatabasePath = stateFilePath(configuration, defaultConfiguration, configuration.BridgeMapDatabasePath, defaultConfiguration.BridgeMapDatabasePath, "bridge-map.sqlite")
	return configuration
}

func stateFilePath(configuration Configuration, defaultConfiguration Configuration, configuredPath string, defaultPath string, fileName string) string {
	if configuredPath != "" {
		return configuredPath
	}
	if configuration.StateDirectory == defaultConfiguration.StateDirectory {
		return defaultPath
	}
	return filepath.Join(configuration.StateDirectory, fileName)
}

func (configuration Configuration) withCredentialDefaults(defaultConfiguration Configuration) Configuration {
	if configuration.MattermostAdminPasswordPath == "" {
		configuration.MattermostAdminPasswordPath = defaultConfiguration.MattermostAdminPasswordPath
	}
	if configuration.MattermostTokenPath == "" {
		configuration.MattermostTokenPath = defaultConfiguration.MattermostTokenPath
	}
	if configuration.OpenRouterKeyPath == "" {
		configuration.OpenRouterKeyPath = defaultConfiguration.OpenRouterKeyPath
	}
	if configuration.OpenRouterModelsURL == "" {
		configuration.OpenRouterModelsURL = defaultConfiguration.OpenRouterModelsURL
	}
	if configuration.ReleaseDownloadTokenPath == "" {
		configuration.ReleaseDownloadTokenPath = defaultConfiguration.ReleaseDownloadTokenPath
	}
	if configuration.MattermostBotTokenPath == "" {
		configuration.MattermostBotTokenPath = defaultConfiguration.MattermostBotTokenPath
	}
	if configuration.AdminEmailPath == "" {
		configuration.AdminEmailPath = defaultConfiguration.AdminEmailPath
	}
	if configuration.ClaimedAdminEmailPath == "" {
		configuration.ClaimedAdminEmailPath = defaultConfiguration.ClaimedAdminEmailPath
	}
	if configuration.APIURLPath == "" {
		configuration.APIURLPath = defaultConfiguration.APIURLPath
	}
	if configuration.APIBaseURL == "" {
		configuration.APIBaseURL = strings.TrimSpace(readTrimmedFile(configuration.APIURLPath))
	}
	return configuration
}

func (configuration Configuration) withCentralPlaneDefaults(defaultConfiguration Configuration) Configuration {
	if configuration.CentralPlaneAppURLPath == "" {
		configuration.CentralPlaneAppURLPath = defaultConfiguration.CentralPlaneAppURLPath
	}
	if configuration.CentralPlaneAgentKeyPath == "" {
		configuration.CentralPlaneAgentKeyPath = defaultConfiguration.CentralPlaneAgentKeyPath
	}
	if configuration.BlueclawAssertionKeyPath == "" {
		configuration.BlueclawAssertionKeyPath = defaultConfiguration.BlueclawAssertionKeyPath
	}
	if configuration.CentralPlaneProjectURLPath == "" {
		configuration.CentralPlaneProjectURLPath = defaultConfiguration.CentralPlaneProjectURLPath
	}
	if configuration.CentralPlanePublishableKeyPath == "" {
		configuration.CentralPlanePublishableKeyPath = defaultConfiguration.CentralPlanePublishableKeyPath
	}
	// A company that hosts internkim itself names its own record. The files are
	// how setup says so, the same way api-url names the fleet; the compiled
	// defaults are only what a device gets when nothing said otherwise.
	if configuration.CentralPlaneProjectURL == "" {
		configuration.CentralPlaneProjectURL = readTrimmedFile(configuration.CentralPlaneProjectURLPath)
	}
	if configuration.CentralPlaneProjectURL == "" {
		configuration.CentralPlaneProjectURL = defaultConfiguration.CentralPlaneProjectURL
	}
	if configuration.CentralPlanePublishableKey == "" {
		configuration.CentralPlanePublishableKey = readTrimmedFile(configuration.CentralPlanePublishableKeyPath)
	}
	if configuration.CentralPlanePublishableKey == "" {
		configuration.CentralPlanePublishableKey = defaultConfiguration.CentralPlanePublishableKey
	}
	if configuration.CentralPlaneAppURL == "" {
		configuration.CentralPlaneAppURL = readTrimmedFile(configuration.CentralPlaneAppURLPath)
	}
	if configuration.CentralPlaneAppURL == "" {
		configuration.CentralPlaneAppURL = companyAddressFrom(configuration.APIBaseURL)
	}
	return configuration
}

func (configuration Configuration) withFleetDefaults(defaultConfiguration Configuration) Configuration {
	if configuration.ReleaseRegistryURL == "" {
		configuration.ReleaseRegistryURL = fleetdomain.Subdomain("updates", fleetdomain.Zone(configuration.APIBaseURL))
	}
	if configuration.FleetIDPath == "" {
		configuration.FleetIDPath = defaultConfiguration.FleetIDPath
	}
	if configuration.DeviceURLPath == "" {
		configuration.DeviceURLPath = defaultConfiguration.DeviceURLPath
	}
	if configuration.BuzzRelayPublicURLPath == "" {
		configuration.BuzzRelayPublicURLPath = defaultConfiguration.BuzzRelayPublicURLPath
	}
	if configuration.TaskPublicURLPath == "" {
		configuration.TaskPublicURLPath = defaultConfiguration.TaskPublicURLPath
	}
	if configuration.FleetSecretPath == "" {
		configuration.FleetSecretPath = defaultConfiguration.FleetSecretPath
	}
	if configuration.AdminUIPath == "" {
		configuration.AdminUIPath = defaultConfiguration.AdminUIPath
	}
	if configuration.RepositoryRoot == "" {
		configuration.RepositoryRoot = defaultConfiguration.RepositoryRoot
	}
	return configuration
}

func (configuration Configuration) withSiteDefaults(defaultConfiguration Configuration) Configuration {
	if configuration.SiteScaffoldPath == "" {
		configuration.SiteScaffoldPath = defaultConfiguration.SiteScaffoldPath
	}
	if configuration.SitesRoot == "" {
		configuration.SitesRoot = defaultConfiguration.SitesRoot
	}
	if configuration.FontsDirectory == "" {
		configuration.FontsDirectory = defaultConfiguration.FontsDirectory
	}
	if configuration.SiteSecretDirectory == "" {
		configuration.SiteSecretDirectory = defaultConfiguration.SiteSecretDirectory
	}
	if configuration.SiteSystemdDirectory == "" {
		configuration.SiteSystemdDirectory = defaultConfiguration.SiteSystemdDirectory
	}
	return configuration
}

func (configuration Configuration) withPersonaDefaults(defaultConfiguration Configuration) Configuration {
	configuration.IdentityDocumentPath = stateFilePath(configuration, defaultConfiguration, configuration.IdentityDocumentPath, defaultConfiguration.IdentityDocumentPath, "identity.json")
	configuration.SoulDocumentPath = stateFilePath(configuration, defaultConfiguration, configuration.SoulDocumentPath, defaultConfiguration.SoulDocumentPath, "soul.json")
	if configuration.BotProfileImagePath == "" {
		configuration.BotProfileImagePath = defaultConfiguration.BotProfileImagePath
	}
	if configuration.BotUsername == "" {
		configuration.BotUsername = defaultConfiguration.BotUsername
	}
	if configuration.BlueclawWorkspacePath == "" {
		configuration.BlueclawWorkspacePath = defaultConfiguration.BlueclawWorkspacePath
	}
	return configuration
}
