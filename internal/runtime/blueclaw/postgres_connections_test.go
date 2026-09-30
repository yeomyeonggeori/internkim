package blueclaw

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestTheConnectionBudgetFitsTheServer(t *testing.T) {
	if PostgresConnectionsGranted() > PostgresConnectionsAvailable() {
		t.Fatalf(
			"the budget grants %d connections against the %d this server leaves after its superuser reserve",
			PostgresConnectionsGranted(),
			PostgresConnectionsAvailable(),
		)
	}
}

func TestTheMessengerIsToldTheShareTheBudgetGivesIt(t *testing.T) {
	messenger, isBundled := CompanyHostServiceNamed(LinuxCompanyHostLayout(), BuzzRelayServiceName)
	if !isBundled {
		t.Fatalf("the bundle carries no %s, so the budget counts connections for a program it cannot see", BuzzRelayServiceName)
	}
	share, isSet := environmentSettingOf(messenger, "BUZZ_DB_POOL_SIZE")
	if !isSet {
		t.Fatal("the messenger unit names no BUZZ_DB_POOL_SIZE, so the budget counts connections it does not control")
	}
	if share != strconv.Itoa(MessengerDatabaseConnections) {
		t.Fatalf("the messenger unit gives the relay %s connections against the %d the budget reserves for it", share, MessengerDatabaseConnections)
	}
}

func environmentSettingOf(service CompanyHostService, name string) (string, bool) {
	for _, source := range service.Environment {
		for _, setting := range source.Settings {
			if setting.Name == name {
				return setting.Value, true
			}
		}
	}
	return "", false
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

func readHostFile(t *testing.T, relativePath string) string {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join("..", "..", "..", relativePath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return strings.TrimSpace(string(document))
}
