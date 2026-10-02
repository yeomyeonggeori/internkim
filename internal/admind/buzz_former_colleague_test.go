package admind

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	nostr "github.com/nbd-wtf/go-nostr"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

func TestAMemberWhoLeftHoldingNoSeatIsShownOutOfTheCommunity(t *testing.T) {
	relay := disposableRelayDatabase(t)
	service := serviceWhoseDirectoryHas(t, `{"members":[
		{"memberID":"member-active","email":"active@example.com","status":"active"},
		{"memberID":"member-departed","email":"departed@example.com","status":"departed","messenger":{"buzz":"`+recordedKeyOfTheDeparted+`"}},
		{"memberID":"member-withdrawn","email":"withdrawn@example.com","status":"withdrawn"}
	]}`)
	ownerKey := derivedKey(t, buzzidentity.BootstrapSubject)
	activeKey := derivedKey(t, versionedSubject("active@example.com", 1))
	departedKey := derivedKey(t, versionedSubject("departed@example.com", 1))
	withdrawnKey := derivedKey(t, versionedSubject("withdrawn@example.com", 1))
	historyAuthorKey := derivedKey(t, versionedSubject("imported-author@example.com", 1))
	holdRelayMembers(t, relay, map[string]string{
		ownerKey:                 "owner",
		activeKey:                "member",
		departedKey:              "member",
		recordedKeyOfTheDeparted: "member",
		withdrawnKey:             "member",
		historyAuthorKey:         "member",
	})

	service.removeSeatsNobodyAccountsFor(context.Background(), relay, nil, identityTestSeed)

	remaining := relayMemberKeys(t, relay)
	expected := []string{ownerKey, activeKey, historyAuthorKey}
	sort.Strings(expected)
	if !reflect.DeepEqual(remaining, expected) {
		t.Fatalf("a member who left must leave the community even without a seat to take back, and nobody else may: got %v, want %v", remaining, expected)
	}
}

const recordedKeyOfTheDeparted = "21ff000000000000000000000000000000000000000000000000000000000001"

func serviceWhoseDirectoryHas(t *testing.T, directory string) *Service {
	t.Helper()
	service := serviceHoldingTheSeed(t)
	agentKeyPath := filepath.Join(t.TempDir(), "agent-key")
	if errorValue := os.WriteFile(agentKeyPath, []byte("agent-key"), 0o600); errorValue != nil {
		t.Fatalf("write agent key: %v", errorValue)
	}
	service.Configuration.CentralPlaneAppURL = "http://central.test"
	service.Configuration.CentralPlaneProjectURL = "http://supabase.test"
	service.Configuration.CentralPlanePublishableKey = "publishable-key"
	service.Configuration.CentralPlaneAgentKeyPath = agentKeyPath
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/agent/member" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, directory, nil), nil
		}
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		}
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	})}
	return service
}

func derivedKey(t *testing.T, subject string) string {
	t.Helper()
	pubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(identityTestSeed, subject))
	if errorValue != nil {
		t.Fatalf("derive %s: %v", subject, errorValue)
	}
	return pubkey
}

func disposableRelayDatabase(t *testing.T) *sql.DB {
	t.Helper()
	connectionString := os.Getenv("BUZZ_RELAY_TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("set BUZZ_RELAY_TEST_DATABASE_URL to a Postgres the test may create a database in")
	}
	administration := openRelayDatabase(t, connectionString)
	databaseName := "relay_" + randomHex(6)
	if _, errorValue := administration.Exec("CREATE DATABASE " + databaseName); errorValue != nil {
		t.Fatalf("create %s: %v", databaseName, errorValue)
	}
	parsed, errorValue := url.Parse(connectionString)
	if errorValue != nil {
		t.Fatalf("parse the relay test database address: %v", errorValue)
	}
	parsed.Path = "/" + databaseName
	relay := openRelayDatabase(t, parsed.String())
	t.Cleanup(func() {
		relay.Close()
		_, _ = administration.Exec("DROP DATABASE " + databaseName)
		administration.Close()
	})
	if _, errorValue := relay.Exec(`CREATE TABLE relay_members (pubkey text PRIMARY KEY, role text NOT NULL)`); errorValue != nil {
		t.Fatalf("create relay_members: %v", errorValue)
	}
	return relay
}

func openRelayDatabase(t *testing.T, connectionString string) *sql.DB {
	t.Helper()
	database, errorValue := sql.Open("postgres", connectionString)
	if errorValue != nil {
		t.Fatalf("open %s: %v", connectionString, errorValue)
	}
	return database
}

func holdRelayMembers(t *testing.T, relay *sql.DB, roleByKey map[string]string) {
	t.Helper()
	for pubkey, role := range roleByKey {
		if _, errorValue := relay.Exec(`INSERT INTO relay_members (pubkey, role) VALUES ($1, $2)`, pubkey, role); errorValue != nil {
			t.Fatalf("hold %s: %v", pubkey, errorValue)
		}
	}
}

func relayMemberKeys(t *testing.T, relay *sql.DB) []string {
	t.Helper()
	rows, errorValue := relay.Query(`SELECT pubkey FROM relay_members ORDER BY pubkey`)
	if errorValue != nil {
		t.Fatalf("read relay_members: %v", errorValue)
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var pubkey string
		if errorValue := rows.Scan(&pubkey); errorValue != nil {
			t.Fatalf("scan relay_members: %v", errorValue)
		}
		keys = append(keys, pubkey)
	}
	return keys
}
