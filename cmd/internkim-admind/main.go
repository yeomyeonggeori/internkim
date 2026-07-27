package main

import (
	"flag"
	"fmt"
	"os"
	_ "time/tzdata"

	"gitlab.com/eastriver/internkim/internal/admind"
)

func main() {
	configuration := admind.DefaultConfiguration()
	flag.StringVar(&configuration.ListenAddress, "listen", configuration.ListenAddress, "HTTP listen address")
	flag.StringVar(&configuration.MattermostBaseURL, "mattermost-url", configuration.MattermostBaseURL, "Mattermost upstream URL")
	flag.StringVar(&configuration.ChatdEndpoint, "chatd-endpoint", configuration.ChatdEndpoint, "chatd platform capability endpoint")
	flag.StringVar(&configuration.ChatdPlatform, "chatd-platform", configuration.ChatdPlatform, "chatd platform name")
	flag.StringVar(&configuration.MattermostTeamName, "mattermost-team", configuration.MattermostTeamName, "Mattermost team name this admind manages")
	flag.StringVar(&configuration.BotUsername, "bot-username", configuration.BotUsername, "Mattermost bot username this admind manages")
	flag.StringVar(&configuration.MattermostPublicURL, "mattermost-public-url", configuration.MattermostPublicURL, "public Mattermost URL for cross-host OAuth authorize (defaults to the request host)")
	flag.StringVar(&configuration.FlowPublicURL, "flow-public-url", configuration.FlowPublicURL, "public Flow web URL for channel open links (defaults to the Mattermost flow base URL)")
	flag.StringVar(&configuration.MattermostAdminPasswordPath, "mattermost-admin-password", configuration.MattermostAdminPasswordPath, "Mattermost admin password path")
	flag.StringVar(&configuration.MattermostInteractiveTokenPath, "mattermost-interactive-token", configuration.MattermostInteractiveTokenPath, "Mattermost interactive action token path")
	flag.StringVar(&configuration.MattermostInteractiveBaseURL, "mattermost-interactive-base-url", configuration.MattermostInteractiveBaseURL, "public base URL for Mattermost interactive actions")
	flag.StringVar(&configuration.BlueclawBaseURL, "blueclaw-url", configuration.BlueclawBaseURL, "Blueclaw upstream URL")
	flag.StringVar(&configuration.APIBaseURL, "api-url", configuration.APIBaseURL, "InternKim Pages API URL")
	flag.StringVar(&configuration.StateDirectory, "state-dir", configuration.StateDirectory, "admin job state directory")
	flag.StringVar(&configuration.FlowDatabasePath, "flow-db", configuration.FlowDatabasePath, "Flow SQLite database path")
	flag.StringVar(&configuration.CalendarDatabasePath, "calendar-db", configuration.CalendarDatabasePath, "calendar SQLite database path")
	flag.StringVar(&configuration.CalendarSecretsDirectory, "calendar-secrets-dir", configuration.CalendarSecretsDirectory, "calendar OAuth secrets directory (contains Google client.json and token store)")
	flag.StringVar(&configuration.MailDatabasePath, "mail-db", configuration.MailDatabasePath, "mail SQLite database path")
	flag.StringVar(&configuration.AttendanceDatabasePath, "attendance-db", configuration.AttendanceDatabasePath, "attendance SQLite database path")
	flag.StringVar(&configuration.BridgeMapDatabasePath, "bridge-map-db", configuration.BridgeMapDatabasePath, "bridge message/channel mapping SQLite database path")
	flag.StringVar(&configuration.AdminEmailPath, "admin-email-path", configuration.AdminEmailPath, "initial admin email file")
	flag.StringVar(&configuration.ClaimedAdminEmailPath, "claimed-admin-email-path", configuration.ClaimedAdminEmailPath, "claimed admin email file")
	flag.StringVar(&configuration.FleetIDPath, "fleet-id-path", configuration.FleetIDPath, "fleet ID file")
	flag.StringVar(&configuration.DeviceURLPath, "device-url-path", configuration.DeviceURLPath, "device public URL file")
	flag.StringVar(&configuration.FleetSecretPath, "fleet-secret-path", configuration.FleetSecretPath, "fleet secret file")
	flag.StringVar(&configuration.OpenRouterKeyPath, "openrouter-key", configuration.OpenRouterKeyPath, "OpenRouter key path")
	flag.StringVar(&configuration.OpenRouterModelsURL, "openrouter-models-url", configuration.OpenRouterModelsURL, "OpenRouter models URL for key validation")
	flag.StringVar(&configuration.ReleaseRegistryURL, "release-registry-url", configuration.ReleaseRegistryURL, "InternKim release registry URL")
	flag.StringVar(&configuration.ReleaseDownloadTokenPath, "release-download-token", configuration.ReleaseDownloadTokenPath, "release registry download token path")
	flag.StringVar(&configuration.ReleaseSigningKeyPath, "release-signing-key", configuration.ReleaseSigningKeyPath, "release manifest signing key path")
	flag.StringVar(&configuration.AdminUIPath, "admin-ui-path", configuration.AdminUIPath, "admin UI static directory")
	flag.StringVar(&configuration.SitesRoot, "sites-root", configuration.SitesRoot, "dynamic sites root directory")
	flag.StringVar(&configuration.SiteSecretDirectory, "site-secret-dir", configuration.SiteSecretDirectory, "dynamic site secret directory")
	flag.StringVar(&configuration.SiteSystemdDirectory, "site-systemd-dir", configuration.SiteSystemdDirectory, "dynamic site systemd directory")
	flag.StringVar(&configuration.BotProfileImagePath, "bot-profile-image", configuration.BotProfileImagePath, "bot profile image path")
	flag.StringVar(&configuration.BlueclawWorkspacePath, "blueclaw-workspace", configuration.BlueclawWorkspacePath, "Blueclaw host workspace path")
	flag.StringVar(&configuration.BuzzInviteKeyPath, "buzz-invite-key", configuration.BuzzInviteKeyPath, "Buzz derived invite HMAC key path (hex)")
	flag.StringVar(&configuration.BuzzCommunityID, "buzz-community-id", configuration.BuzzCommunityID, "Buzz community UUID invites admit into")
	flag.StringVar(&configuration.BuzzRelayURL, "buzz-relay-url", configuration.BuzzRelayURL, "Buzz relay WebSocket URL for deep links")
	flag.StringVar(&configuration.BuzzLandingBaseURL, "buzz-landing-url", configuration.BuzzLandingBaseURL, "Buzz invite landing page base URL")
	flag.StringVar(&configuration.BuzzAdminCommandPath, "buzz-admin-command", configuration.BuzzAdminCommandPath, "buzz-admin binary path for member polling")
	flag.StringVar(&configuration.BuzzDatabaseURL, "buzz-database-url", configuration.BuzzDatabaseURL, "Buzz relay postgres URL for member polling")
	flag.StringVar(&configuration.BuzzAccountLinksPath, "buzz-account-links", configuration.BuzzAccountLinksPath, "account links JSON file consumed by acpd")
	flag.Parse()

	if errorValue := admind.Run(configuration); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
}
