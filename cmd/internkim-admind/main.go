package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	_ "time/tzdata"

	"github.com/yeomyeonggeori/internkim/internal/admind"
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
	flag.StringVar(&configuration.ListenSocketPath, "listen-socket", configuration.ListenSocketPath, "unix socket the relay reaches this daemon on, and the only door an asserted requester arrives through")
	flag.StringVar(&configuration.BlueclawPolicyDeliveryPath, "blueclaw-policy", configuration.BlueclawPolicyDeliveryPath, "policy document this daemon reconciles the company roster onto")
	flag.StringVar(&configuration.CapabilitySocketPath, "capability-socket", configuration.CapabilitySocketPath, "capabilityd unix socket path")
	flag.StringVar(&configuration.ChatdEndpoint, "chatd-endpoint", configuration.ChatdEndpoint, "chatd platform capability endpoint")
	flag.StringVar(&configuration.ChatdPlatform, "chatd-platform", configuration.ChatdPlatform, "chatd platform name")
	flag.StringVar(&configuration.MattermostTeamName, "mattermost-team", configuration.MattermostTeamName, "Mattermost team name this admind manages")
	flag.StringVar(&configuration.BotUsername, "bot-username", configuration.BotUsername, "Mattermost bot username this admind manages")
	flag.StringVar(&configuration.TaskPublicURL, "flow-public-url", configuration.TaskPublicURL, "public Flow web URL for channel open links (defaults to the Mattermost flow base URL)")
	flag.StringVar(&configuration.MattermostAdminPasswordPath, "mattermost-admin-password", configuration.MattermostAdminPasswordPath, "Mattermost admin password path")
	flag.StringVar(&configuration.BlueclawBaseURL, "blueclaw-url", configuration.BlueclawBaseURL, "Blueclaw upstream URL")
	flag.StringVar(&configuration.APIBaseURL, "api-url", configuration.APIBaseURL, "internkim Pages API URL")
	flag.StringVar(&configuration.StateDirectory, "state-dir", configuration.StateDirectory, "admin job state directory")
	flag.StringVar(&configuration.DatabasePath, "database", configuration.DatabasePath, "unified SQLite database path")
	flag.StringVar(&configuration.TaskDatabasePath, "flow-db", configuration.TaskDatabasePath, "Flow SQLite database path")
	flag.StringVar(&configuration.MailDatabasePath, "mail-db", configuration.MailDatabasePath, "mail SQLite database path")
	flag.StringVar(&configuration.AttendanceDatabasePath, "attendance-db", configuration.AttendanceDatabasePath, "attendance SQLite database path")
	flag.StringVar(&configuration.CentralPlaneAppURL, "central-plane-app-url", configuration.CentralPlaneAppURL, "central plane app URL, which issues member sessions")
	flag.StringVar(&configuration.CentralPlaneAgentKeyPath, "central-plane-agent-key", configuration.CentralPlaneAgentKeyPath, "file holding the credential this company computer presents to the central plane")
	flag.StringVar(&configuration.BlueclawAssertionKeyPath, "blueclaw-assertion-key", configuration.BlueclawAssertionKeyPath, "file holding the key that signs what admind asks Blueclaw for on a person's behalf")
	flag.StringVar(&configuration.CentralPlaneProjectURL, "central-plane-project-url", configuration.CentralPlaneProjectURL, "central plane project URL")
	flag.StringVar(&configuration.CentralPlanePublishableKey, "central-plane-publishable-key", configuration.CentralPlanePublishableKey, "central plane publishable key")
	flag.StringVar(&configuration.BridgeMapDatabasePath, "bridge-map-db", configuration.BridgeMapDatabasePath, "bridge message/channel mapping SQLite database path")
	flag.StringVar(&configuration.AdminEmailPath, "admin-email-path", configuration.AdminEmailPath, "initial admin email file")
	flag.StringVar(&configuration.ClaimedAdminEmailPath, "claimed-admin-email-path", configuration.ClaimedAdminEmailPath, "claimed admin email file")
	flag.StringVar(&configuration.TaskPublicURLPath, "flow-public-url-path", configuration.TaskPublicURLPath, "public Flow web URL file for channel open links")
	flag.StringVar(&configuration.OpenRouterKeyPath, "openrouter-key", configuration.OpenRouterKeyPath, "OpenRouter key path")
	flag.StringVar(&configuration.OpenRouterModelsURL, "openrouter-models-url", configuration.OpenRouterModelsURL, "OpenRouter models URL for key validation")
	flag.StringVar(&configuration.AdminUIPath, "admin-ui-path", configuration.AdminUIPath, "admin UI static directory")
	flag.StringVar(&configuration.BotProfileImagePath, "bot-profile-image", configuration.BotProfileImagePath, "bot profile image path")
	flag.StringVar(&configuration.BlueclawWorkspacePath, "blueclaw-workspace", configuration.BlueclawWorkspacePath, "Blueclaw host workspace path")
	flag.StringVar(&configuration.BuzzInviteKeyPath, "buzz-invite-key", configuration.BuzzInviteKeyPath, "Buzz derived invite HMAC key path (hex)")
	flag.StringVar(&configuration.BuzzCommunityID, "buzz-community-id", configuration.BuzzCommunityID, "Buzz community UUID invites admit into")
	flag.StringVar(&configuration.BuzzRelayURL, "buzz-relay-url", configuration.BuzzRelayURL, "Buzz relay WebSocket URL used to connect from inside the box (loopback path)")
	flag.StringVar(&configuration.BuzzRelayPublicURL, "buzz-relay-public-url", configuration.BuzzRelayPublicURL, "public wss URL for the relay, e.g. wss://relay.example.test; used for invite deep links, browser relay config, and the Host header presented to the relay")
	flag.StringVar(&configuration.BuzzAdminCommandPath, "buzz-admin-command", configuration.BuzzAdminCommandPath, "buzz-admin binary path for member polling")
	flag.StringVar(&configuration.BuzzDatabaseURL, "buzz-database-url", configuration.BuzzDatabaseURL, "Buzz relay postgres URL for member polling")
	buzzDatabaseURLPath := flag.String("buzz-database-url-path", "", "file holding the Buzz relay postgres URL (EnvironmentFile format); read when -buzz-database-url is empty")
	flag.StringVar(&configuration.BuzzAccountLinksPath, "buzz-account-links", configuration.BuzzAccountLinksPath, "account links JSON file consumed by acpd")
	flag.StringVar(&configuration.BuzzKeySeedPath, "buzz-key-seed-path", configuration.BuzzKeySeedPath, "file holding the Buzz identity derivation seed (must match the history importer)")
	flag.StringVar(&configuration.BuzzRelayKeyPath, "buzz-relay-key-path", configuration.BuzzRelayKeyPath, "EnvironmentFile holding BUZZ_RELAY_PRIVATE_KEY for buzz-admin relay membership grants")
	flag.BoolVar(&configuration.TrustProxyForwardedEmail, "trust-proxy-forwarded-email", configuration.TrustProxyForwardedEmail, "trust the X-Forwarded-Email/X-Auth-Request-Email header from a fronting identity proxy (oauth2-proxy, Authelia); enable only when such a proxy is the sole ingress")
	flag.BoolVar(&configuration.TaskRunNotifyEnabled, "task-run-notify", configuration.TaskRunNotifyEnabled, "push a notification to the central plane when a task run needs approval, completes, or fails")
	flag.BoolVar(&configuration.AttendanceNotifyEnabled, "attendance-notify", configuration.AttendanceNotifyEnabled, "push a notification when somebody clocks in or out, and when a leave request reaches the administrators")
	flag.BoolVar(&configuration.MailNotifyEnabled, "mail-notify", configuration.MailNotifyEnabled, "push a notification when unread mail arrives for somebody with a connected account")
	flag.Parse()

	if configuration.BuzzDatabaseURL == "" && *buzzDatabaseURLPath != "" {
		databaseURL, errorValue := readBuzzDatabaseURL(*buzzDatabaseURLPath)
		switch {
		case errorValue == nil:
			configuration.BuzzDatabaseURL = databaseURL
		case errors.Is(errorValue, os.ErrNotExist):
			// A deployment without the Buzz relay has no relay database, and
			// admind serves everything else without one.
			fmt.Fprintf(os.Stderr, "buzz relay database is not configured at %s; continuing without buzz\n", *buzzDatabaseURLPath)
		default:
			fmt.Fprintln(os.Stderr, errorValue)
			os.Exit(1)
		}
	}

	if errorValue := admind.Run(configuration); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
}
