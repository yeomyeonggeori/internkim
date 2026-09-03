package setup

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type relayDatabaseConnection struct {
	databaseComesUp bool
	cacheStaysDown  bool
	commands        []string
}

func (connection *relayDatabaseConnection) Run(command string) string {
	connection.commands = append(connection.commands, command)
	if command == "systemctl is-active redis-server" {
		if connection.cacheStaysDown {
			return "inactive\n"
		}
		return "active\n"
	}
	if command == "systemctl is-active postgresql" {
		if connection.databaseComesUp {
			return "active\n"
		}
		return "inactive\n"
	}
	if strings.Contains(command, "apt-get install") {
		return "E: Unable to locate package postgresql"
	}
	return ""
}

func (connection *relayDatabaseConnection) SCP(localPath, remotePath string) error {
	return nil
}

func relayContext(connection BoardConnection) *Context {
	return &Context{
		Backend: BackendSSH,
		SSH:     connection,
		Callbacks: Callbacks{
			GetBuzzRelayOwnerPubkey:     func() (string, error) { return strings.Repeat("a", 64), nil },
			InstallBuzzRelayBinariesSSH: func(context *Context) error { return nil },
		},
	}
}

func TestBuzzRelayInstallsTheDatabaseItBindsTo(t *testing.T) {
	connection := &relayDatabaseConnection{databaseComesUp: true}
	if errorValue := StepBuzzRelay.Run(relayContext(connection)); errorValue != nil {
		t.Fatalf("a machine whose postgres comes up must install cleanly, got %v", errorValue)
	}
	installed := strings.Join(connection.commands, "\n")
	if !strings.Contains(installed, "apt-get install -y -qq "+blueclaw.BuzzRelayDatabasePackages) {
		t.Fatalf("the step must install the packages the image path downloads, got %s", installed)
	}
}

func TestBuzzRelayFailsWhenTheDatabaseDoesNotStart(t *testing.T) {
	errorValue := StepBuzzRelay.Run(relayContext(&relayDatabaseConnection{databaseComesUp: false}))
	if errorValue == nil {
		t.Fatal("a relay bound to a postgres that is not running must not report success")
	}
	if !strings.Contains(errorValue.Error(), "postgresql") {
		t.Fatalf("the failure must name what is missing, got %v", errorValue)
	}
}

func TestBuzzRelayFailsWhenTheCacheDoesNotStart(t *testing.T) {
	errorValue := StepBuzzRelay.Run(relayContext(&relayDatabaseConnection{databaseComesUp: true, cacheStaysDown: true}))
	if errorValue == nil {
		t.Fatal("a relay whose redis is not running must not report success")
	}
	if !strings.Contains(errorValue.Error(), "redis-server") {
		t.Fatalf("the failure must name what is missing, got %v", errorValue)
	}
}

func TestBuzzRelayWaitsForTheAccountsItsPackagesCreate(t *testing.T) {
	connection := &relayDatabaseConnection{databaseComesUp: true}
	if errorValue := StepBuzzRelay.Run(relayContext(connection)); errorValue != nil {
		t.Fatalf("a machine whose services come up must install cleanly, got %v", errorValue)
	}
	for _, packageName := range []string{"redis-server", blueclaw.BuzzRelayDatabasePackages} {
		installCommand := ""
		for _, command := range connection.commands {
			if strings.Contains(command, "apt-get install -y -qq "+packageName) {
				installCommand = command
			}
		}
		if installCommand == "" {
			t.Fatalf("expected the step to install %s", packageName)
		}
		if !strings.Contains(installCommand, "wait_for_package_work_to_settle") {
			t.Fatalf("installing %s must not return while useradd is still rewriting /etc/shadow: %s", packageName, installCommand)
		}
	}
}

func TestBuzzRelayUnitBindsToTheDatabaseTheStepInstalls(t *testing.T) {
	if !strings.Contains(blueclaw.BuzzRelayServiceUnit(""), "BindsTo=postgresql.service") {
		t.Fatal("the step installs postgres because the unit binds to it; one moving without the other is the bug")
	}
}

func TestChatdHealthCheckAsksForTheRouteChatdServes(t *testing.T) {
	command := blueclaw.ChatdHealthCheckCommand()
	if !strings.Contains(command, blueclaw.ChatdEndpoint+blueclaw.ChatdHealthPath) {
		t.Fatalf("the health check must curl the one path chatd answers, got %s", command)
	}
}
