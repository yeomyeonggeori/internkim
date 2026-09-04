package admind

import (
	"path/filepath"

	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"

	"gitlab.com/eastriver/internkim/internal/centralplane"
	"gitlab.com/eastriver/internkim/internal/fleetdomain"
	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
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
	CompanionJobPath               string
	UsersSyncStatePath             string
	DatabasePath                   string
	TaskDatabasePath               string
	CalendarDatabasePath           string
	MailDatabasePath               string
	AttendanceDatabasePath         string
	CentralPlaneAppURL             string
	CentralPlaneAgentKeyPath       string
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
	ReleaseSigningKeyPath          string
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
	CompanionFileDirectory         string
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
}

func DefaultConfiguration() Configuration {
	return Configuration{
		ListenAddress:                  "127.0.0.1:18080",
		ListenSocketPath:               blueclawruntime.AdmindSocketPath,
		TaskRunNotifyEnabled:           true,
		AttendanceNotifyEnabled:        true,
		MailNotifyEnabled:              true,
		MattermostBaseURL:              "http://127.0.0.1:8065",
		MattermostTeamName:             "internkim",
		BotUsername:                    "internkim",
		BlueclawBaseURL:                "http://127.0.0.1:8080",
		BlueclawPolicyDeliveryPath:     filepath.Join(blueclaw.BlueclawDeliveryConfigPath, "policy.json"),
		CapabilitySocketPath:           blueclawruntime.CapabilitySocketPath,
		StateDirectory:                 "/root/.internkim/state/admin",
		CompanionJobPath:               "/root/.internkim/state/companion-jobs.json",
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
		ReleaseSigningKeyPath:          "/root/.internkim/secrets/release-signing-key",
		MattermostBotTokenPath:         "/root/.internkim/secrets/mattermost-bot-token",
		AdminEmailPath:                 "/root/.internkim/config/admin-email",
		ClaimedAdminEmailPath:          "/root/.internkim/state/admin/claimed-admin-email",
		APIURLPath:                     "/root/.internkim/env/api-url",
		CentralPlaneAgentKeyPath:       "/root/.internkim/secrets/central-plane-agent-key",
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
		CompanionFileDirectory:         "/tmp/internkim-companion-files",
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
	if configuration.CompanionJobPath == "" {
		if configuration.StateDirectory == defaultConfiguration.StateDirectory {
			configuration.CompanionJobPath = defaultConfiguration.CompanionJobPath
		} else {
			configuration.CompanionJobPath = filepath.Join(configuration.StateDirectory, "companion-jobs.json")
		}
	}
	if configuration.TaskDatabasePath == "" {
		if configuration.CompanionJobPath == defaultConfiguration.CompanionJobPath {
			configuration.TaskDatabasePath = defaultConfiguration.TaskDatabasePath
		} else {
			configuration.TaskDatabasePath = filepath.Join(filepath.Dir(configuration.CompanionJobPath), "flow.sqlite")
		}
	}
	if configuration.CalendarDatabasePath == "" {
		if configuration.CompanionJobPath == defaultConfiguration.CompanionJobPath {
			configuration.CalendarDatabasePath = defaultConfiguration.CalendarDatabasePath
		} else {
			configuration.CalendarDatabasePath = filepath.Join(filepath.Dir(configuration.CompanionJobPath), "calendar.sqlite")
		}
	}
	if configuration.MailDatabasePath == "" {
		if configuration.CompanionJobPath == defaultConfiguration.CompanionJobPath {
			configuration.MailDatabasePath = defaultConfiguration.MailDatabasePath
		} else {
			configuration.MailDatabasePath = filepath.Join(filepath.Dir(configuration.CompanionJobPath), "mail.sqlite")
		}
	}
	if configuration.AttendanceDatabasePath == "" {
		if configuration.CompanionJobPath == defaultConfiguration.CompanionJobPath {
			configuration.AttendanceDatabasePath = defaultConfiguration.AttendanceDatabasePath
		} else {
			configuration.AttendanceDatabasePath = filepath.Join(filepath.Dir(configuration.CompanionJobPath), "attendance.sqlite")
		}
	}
	if configuration.BridgeMapDatabasePath == "" {
		if configuration.CompanionJobPath == defaultConfiguration.CompanionJobPath {
			configuration.BridgeMapDatabasePath = defaultConfiguration.BridgeMapDatabasePath
		} else {
			configuration.BridgeMapDatabasePath = filepath.Join(filepath.Dir(configuration.CompanionJobPath), "bridge-map.sqlite")
		}
	}
	return configuration
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
	if configuration.ReleaseSigningKeyPath == "" {
		configuration.ReleaseSigningKeyPath = defaultConfiguration.ReleaseSigningKeyPath
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
	if configuration.CompanionFileDirectory == "" {
		configuration.CompanionFileDirectory = defaultConfiguration.CompanionFileDirectory
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
	if configuration.IdentityDocumentPath == "" {
		configuration.IdentityDocumentPath = personaDocumentPath(configuration, defaultConfiguration, defaultConfiguration.IdentityDocumentPath, "identity.json")
	}
	if configuration.SoulDocumentPath == "" {
		configuration.SoulDocumentPath = personaDocumentPath(configuration, defaultConfiguration, defaultConfiguration.SoulDocumentPath, "soul.json")
	}
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

func personaDocumentPath(configuration Configuration, defaultConfiguration Configuration, defaultPath string, fileName string) string {
	if configuration.CompanionJobPath == defaultConfiguration.CompanionJobPath {
		return defaultPath
	}
	return filepath.Join(filepath.Dir(configuration.CompanionJobPath), fileName)
}
