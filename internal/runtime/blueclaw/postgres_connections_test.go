package blueclaw

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// A compose file read as text answers "does this string appear somewhere",
// which a comment, a second service, or an entry on the wrong service all
// satisfy while the server and the messenger run on something else. These
// decode it and ask the service that actually carries the setting.
type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Command     []string       `yaml:"command"`
	Environment map[string]any `yaml:"environment"`
}

func TestTheConnectionBudgetFitsTheServer(t *testing.T) {
	if PostgresConnectionsGranted() > PostgresConnectionsAvailable() {
		t.Fatalf(
			"the budget grants %d connections against the %d this server leaves after its superuser reserve",
			PostgresConnectionsGranted(),
			PostgresConnectionsAvailable(),
		)
	}
}

func TestTheServerIsToldTheCapacityTheBudgetDividesUp(t *testing.T) {
	postgres := quickstartService(t, "postgres")
	capacity, isSet := settingPostgresIsStartedWith(postgres.Command, "max_connections")
	if !isSet {
		t.Fatalf("the quickstart starts postgres with %v, which sets no max_connections, so the budget divides up a number nobody set", postgres.Command)
	}
	if capacity != strconv.Itoa(PostgresMaxConnections) {
		t.Fatalf("the quickstart starts postgres with max_connections=%s against a budget that divides up %d", capacity, PostgresMaxConnections)
	}
}

func TestTheMessengerIsToldTheShareTheBudgetGivesIt(t *testing.T) {
	messenger := quickstartService(t, "messenger")
	share, isSet := messenger.Environment["BUZZ_DB_POOL_SIZE"]
	if !isSet {
		t.Fatal("the messenger service's environment names no BUZZ_DB_POOL_SIZE, so the budget counts connections it does not control")
	}
	if fmt.Sprintf("%v", share) != strconv.Itoa(MessengerDatabaseConnections) {
		t.Fatalf("the quickstart gives the messenger %v connections against the %d the budget reserves for it", share, MessengerDatabaseConnections)
	}
}

func TestTheAgentIsToldTheShareTheBudgetGivesIt(t *testing.T) {
	share, isSet := databaseConnectionShareOf(t, readHostFile(t, filepath.Join("host", "runtime.template.json")))
	if !isSet {
		t.Fatalf("the host runtime template names no database.%s, so the agent falls back to everything the server allows", AgentDatabaseConnectionsField)
	}
	if share != AgentDatabaseConnections {
		t.Fatalf("the host runtime template gives the agent %d connections against the %d the budget reserves for it", share, AgentDatabaseConnections)
	}
}

// A device is not rendered from the host template, it is rendered here. Nothing
// bound this before, so deleting the field left every test passing and every
// device asking the server for everything it allows.
func TestTheDeviceRuntimeDocumentIsToldTheShareTheBudgetGivesIt(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{
		DatabaseConnectionString: "postgres://internkim@postgres/tenant_01?sslmode=disable",
		WorkspaceRootPath:        "/workspace",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	share, isSet := databaseConnectionShareOf(t, document)
	if !isSet {
		t.Fatalf("the device runtime document names no database.%s, so a provisioned device falls back to everything the server allows", AgentDatabaseConnectionsField)
	}
	if share != AgentDatabaseConnections {
		t.Fatalf("the device runtime document gives the agent %d connections against the %d the budget reserves for it", share, AgentDatabaseConnections)
	}
}

// blueclaw is a separate Go module under a different licence, so the share can
// only reach it as a field name written twice. blueclaw's committed device
// example is the shape its own loader is tested against
// (internal/config/connection_budget_wire_test.go there), so a rename on either
// side stops matching here.
func TestTheAgentShareTravelsUnderTheNameBlueclawReads(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Join("..", "..", "..", ".dependency", "blueclaw", "config", "runtime.example.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	share, isSet := databaseConnectionShareOf(t, string(document))
	if !isSet {
		t.Fatalf("blueclaw's device example names no database.%s, so the name this renderer writes is not the name blueclaw reads", AgentDatabaseConnectionsField)
	}
	if share != AgentDatabaseConnections {
		t.Fatalf("blueclaw's device example carries %d connections against the %d the budget reserves for the agent", share, AgentDatabaseConnections)
	}
}

func databaseConnectionShareOf(t *testing.T, document string) (int, bool) {
	t.Helper()
	var decoded struct {
		Database map[string]json.RawMessage `json:"database"`
	}
	if errorValue := json.Unmarshal([]byte(document), &decoded); errorValue != nil {
		t.Fatal(errorValue)
	}
	rawShare, isSet := decoded.Database[AgentDatabaseConnectionsField]
	if !isSet {
		return 0, false
	}
	var share int
	if errorValue := json.Unmarshal(rawShare, &share); errorValue != nil {
		t.Fatal(errorValue)
	}
	return share, true
}

// postgres takes its settings as "-c name=value" pairs, so a value that is not
// the argument after a -c is not a value postgres was started with.
func settingPostgresIsStartedWith(command []string, name string) (string, bool) {
	value := ""
	isSet := false
	for index := 0; index+1 < len(command); index++ {
		if command[index] != "-c" {
			continue
		}
		setting, found := strings.CutPrefix(command[index+1], name+"=")
		if !found {
			continue
		}
		value, isSet = setting, true
	}
	return value, isSet
}

func quickstartService(t *testing.T, name string) composeService {
	t.Helper()
	var compose composeFile
	if errorValue := yaml.Unmarshal([]byte(readHostFile(t, filepath.Join("host", "quickstart", "compose.yaml"))), &compose); errorValue != nil {
		t.Fatal(errorValue)
	}
	service, isDeclared := compose.Services[name]
	if !isDeclared {
		t.Fatalf("the quickstart declares no %s service, so the budget divides connections between programs it cannot see", name)
	}
	return service
}

func readHostFile(t *testing.T, relativePath string) string {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join("..", "..", "..", relativePath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return strings.TrimSpace(string(document))
}
