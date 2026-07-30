package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	_ "time/tzdata"

	"gitlab.com/eastriver/internkim/internal/admind"
)

func readBuzzDatabaseURL(path string) (string, error) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return "", fmt.Errorf("read buzz database url from %s: %w", path, errorValue)
	}
	for _, line := range strings.Split(string(document), "\n") {
		value, found := strings.CutPrefix(strings.TrimSpace(line), "DATABASE_URL=")
		if !found {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if value != "" {
			return value, nil
		}
	}
	return "", fmt.Errorf("no DATABASE_URL found in %s", path)
}

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
	flag.StringVar(&configuration.DatabasePath, "database", configuration.DatabasePath, "unified SQLite database path")
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
	flag.StringVar(&configuration.BuzzRelayURL, "buzz-relay-url", configuration.BuzzRelayURL, "Buzz relay WebSocket URL used to connect from inside the box (loopback path)")
	flag.StringVar(&configuration.BuzzRelayPublicURL, "buzz-relay-public-url", configuration.BuzzRelayPublicURL, "public wss URL for the relay, e.g. wss://<tenant>-relay.intern.kim; used for invite deep links, browser relay config, and the Host header presented to the relay")
	flag.StringVar(&configuration.BuzzLandingBaseURL, "buzz-landing-url", configuration.BuzzLandingBaseURL, "Buzz invite landing page base URL")
	flag.StringVar(&configuration.BuzzAdminCommandPath, "buzz-admin-command", configuration.BuzzAdminCommandPath, "buzz-admin binary path for member polling")
	flag.StringVar(&configuration.BuzzDatabaseURL, "buzz-database-url", configuration.BuzzDatabaseURL, "Buzz relay postgres URL for member polling")
	buzzDatabaseURLPath := flag.String("buzz-database-url-path", "", "file holding the Buzz relay postgres URL (EnvironmentFile format); read when -buzz-database-url is empty")
	flag.StringVar(&configuration.BuzzAccountLinksPath, "buzz-account-links", configuration.BuzzAccountLinksPath, "account links JSON file consumed by acpd")
	flag.StringVar(&configuration.BuzzKeySeedPath, "buzz-key-seed-path", configuration.BuzzKeySeedPath, "file holding the Buzz identity derivation seed (must match the history importer)")
	flag.StringVar(&configuration.BuzzRelayKeyPath, "buzz-relay-key-path", configuration.BuzzRelayKeyPath, "EnvironmentFile holding BUZZ_RELAY_PRIVATE_KEY for buzz-admin relay membership grants")
	flag.StringVar(&configuration.CloudflareAccessTeamDomain, "cloudflare-access-team-domain", configuration.CloudflareAccessTeamDomain, "Cloudflare Access team domain (e.g. example.cloudflareaccess.com) whose JWT the web trusts")
	flag.StringVar(&configuration.CloudflareAccessAUDs, "cloudflare-access-aud", configuration.CloudflareAccessAUDs, "comma-separated Cloudflare Access application AUD tags the web session accepts")
	flag.BoolVar(&configuration.TrustProxyForwardedEmail, "trust-proxy-forwarded-email", configuration.TrustProxyForwardedEmail, "trust the X-Forwarded-Email/X-Auth-Request-Email header from a fronting identity proxy (oauth2-proxy, Authelia); enable only when such a proxy is the sole ingress")
	flag.Parse()

	if configuration.BuzzDatabaseURL == "" && *buzzDatabaseURLPath != "" {
		databaseURL, errorValue := readBuzzDatabaseURL(*buzzDatabaseURLPath)
		switch {
		case errors.Is(errorValue, os.ErrNotExist):
			fmt.Fprintf(os.Stderr, "buzz database url file %s not present yet; continuing without buzz member polling\n", *buzzDatabaseURLPath)
		case errorValue != nil:
			fmt.Fprintln(os.Stderr, errorValue)
			os.Exit(1)
		default:
			configuration.BuzzDatabaseURL = databaseURL
		}
	}

	if errorValue := admind.Run(configuration); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
}
