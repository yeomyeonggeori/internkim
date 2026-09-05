package setup

import (
	"slices"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestBuzzChatdStartsAfterItsGuestGatewayExists(t *testing.T) {
	for _, selector := range []Selector{{}, {Only: []string{"buzz-chatd"}}} {
		selector.DryRun = true
		plan, errorValue := DefaultRegistry().resolve(&Context{Backend: BackendSSH}, selector)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		servicesIndex := slices.Index(plan, "services")
		chatdIndex := slices.Index(plan, "buzz-chatd")
		if servicesIndex < 0 || chatdIndex <= servicesIndex {
			t.Fatalf("chatd must follow the services that create its gateway address: %v", plan)
		}
	}
}

const chatdTestAgentSecret = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

type chatdConnection struct{ installedEnvironment string }

func (connection chatdConnection) Run(command string) string {
	switch {
	case strings.HasPrefix(command, "systemctl is-active"):
		return "active"
	case command == blueclaw.ChatdHealthCheckCommand():
		return "ok"
	case command == "cat "+blueclaw.ChatdServicePath:
		return "CHATD_LISTEN_HOSTNAME=" + blueclaw.ChatdListenHostname
	case command == "cat "+blueclaw.ChatdEnvironmentFilePath:
		return connection.installedEnvironment
	}
	return ""
}

func (connection chatdConnection) SCP(localPath, remotePath string) error {
	return nil
}

func chatdContext(installedEnvironment string) *Context {
	return &Context{
		Backend: BackendSSH,
		SSH:     chatdConnection{installedEnvironment: installedEnvironment},
		Callbacks: Callbacks{
			GetBuzzAgentSecret: func() (string, error) { return chatdTestAgentSecret, nil },
		},
	}
}

func TestBuzzChatdIsSatisfiedByTheEnvironmentItWrites(t *testing.T) {
	if !StepBuzzChatd.IsSatisfied(chatdContext(chatdEnvironmentFileContents(chatdTestAgentSecret))) {
		t.Fatal("the environment this step writes must count as installed")
	}
}

func TestBuzzChatdReinstallsWhenTheEnvironmentHasDrifted(t *testing.T) {
	stale := chatdEnvironmentFileContents(chatdTestAgentSecret) +
		"CHATD_BUZZ_KEY_SEED_PATH=/root/.internkim/secrets/buzz-key-seed\n"
	if StepBuzzChatd.IsSatisfied(chatdContext(stale)) {
		t.Fatal("a line this step no longer writes must require reinstallation")
	}
}

func TestBuzzChatdReinstallsWhenTheAgentSecretHasChanged(t *testing.T) {
	if StepBuzzChatd.IsSatisfied(chatdContext(chatdEnvironmentFileContents(strings.Repeat("0", 64)))) {
		t.Fatal("an environment naming another key must require reinstallation")
	}
}

func TestBuzzChatdEnvironmentNamesNoIdentitySeed(t *testing.T) {
	command := chatdEnvironmentCommand(chatdTestAgentSecret)
	if strings.Contains(command, "CHATD_BUZZ_KEY_SEED_PATH") {
		t.Fatal("chatd no longer derives identities, so it is handed no identity seed")
	}
	if !strings.Contains(command, "CHATD_BUZZ_PRIVATE_KEY="+chatdTestAgentSecret) {
		t.Fatalf("the agent's own key must still be written, got %s", command)
	}
}
