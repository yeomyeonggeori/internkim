package blueclaw

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

func TestTheServerIsToldTheCapacityTheBudgetDividesUp(t *testing.T) {
	composeDocument := readHostFile(t, filepath.Join("host", "quickstart", "compose.yaml"))
	capacity := regexp.MustCompile(`max_connections=(\d+)`).FindStringSubmatch(composeDocument)
	if capacity == nil {
		t.Fatal("the quickstart never tells postgres how many connections to allow, so the budget divides up a number nobody set")
	}
	if capacity[1] != strconv.Itoa(PostgresMaxConnections) {
		t.Fatalf("the quickstart runs postgres with max_connections=%s against a budget that divides up %d", capacity[1], PostgresMaxConnections)
	}
}

func TestTheMessengerIsToldTheShareTheBudgetGivesIt(t *testing.T) {
	composeDocument := readHostFile(t, filepath.Join("host", "quickstart", "compose.yaml"))
	share := regexp.MustCompile(`BUZZ_DB_POOL_SIZE:\s*"?(\d+)"?`).FindStringSubmatch(composeDocument)
	if share == nil {
		t.Fatal("the quickstart never tells the messenger its share, so the budget counts connections it does not control")
	}
	if share[1] != strconv.Itoa(MessengerDatabaseConnections) {
		t.Fatalf("the quickstart gives the messenger %s connections against the %d the budget reserves for it", share[1], MessengerDatabaseConnections)
	}
}

func TestTheAgentIsToldTheShareTheBudgetGivesIt(t *testing.T) {
	var template struct {
		Database struct {
			MaxOpenConnections *int `json:"maxOpenConnections"`
		} `json:"database"`
	}
	if errorValue := json.Unmarshal([]byte(readHostFile(t, filepath.Join("host", "runtime.template.json"))), &template); errorValue != nil {
		t.Fatal(errorValue)
	}
	if template.Database.MaxOpenConnections == nil {
		t.Fatal("the host runtime template never tells the agent its share, so the agent falls back to everything the server allows")
	}
	if *template.Database.MaxOpenConnections != AgentDatabaseConnections {
		t.Fatalf("the host runtime template gives the agent %d connections against the %d the budget reserves for it", *template.Database.MaxOpenConnections, AgentDatabaseConnections)
	}
}

func readHostFile(t *testing.T, relativePath string) string {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join("..", "..", "..", relativePath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return strings.TrimSpace(string(document))
}
