package setup

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

func TestMattermostManagedResourceCheckRequiresAllManagedPaths(t *testing.T) {
	command := mattermostManagedResourceCheckCommand()
	for _, resourcePath := range mattermostdefaults.ManagedResourcePaths() {
		if !strings.Contains(command, `"`+resourcePath+`"`) {
			t.Fatalf("expected Mattermost check command to include %s, got:\n%s", resourcePath, command)
		}
	}
}

func TestMattermostManagedResourceCheckRequiresTokenAndBotSettings(t *testing.T) {
	command := mattermostManagedResourceCheckCommand()
	for _, setting := range []string{"EnableUserAccessTokens", "EnableBotAccountCreation", "AllowedUntrustedInternalConnections"} {
		if !strings.Contains(command, setting) {
			t.Fatalf("expected Mattermost check command to include %s, got:\n%s", setting, command)
		}
	}
	for _, value := range []string{"127.0.0.1", "localhost"} {
		if !strings.Contains(command, value) {
			t.Fatalf("expected Mattermost check command to include %s, got:\n%s", value, command)
		}
	}
}
