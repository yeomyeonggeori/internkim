package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type carriedMailAccount struct {
	Signer  string
	Written map[string]any
}

func startPlaneKnowing(t *testing.T, known []string) (*httptest.Server, *[]carriedMailAccount) {
	t.Helper()
	written := []carriedMailAccount{}
	knownAddresses := map[string]bool{}
	for _, address := range known {
		knownAddresses[address] = true
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/api/agent/session":
			var asked struct {
				ExternalID string `json:"externalID"`
			}
			_ = json.NewDecoder(request.Body).Decode(&asked)
			if !knownAddresses[asked.ExternalID] {
				http.Error(writer, "no such member", http.StatusNotFound)
				return
			}
			_, _ = writer.Write([]byte(`{"memberID":"member-` + asked.ExternalID + `","accessToken":"token-` + asked.ExternalID + `","expiresAt":99999999999}`))
		case request.URL.Path == "/api/member/mail-account" && request.Method == http.MethodPut:
			var body map[string]any
			_ = json.NewDecoder(request.Body).Decode(&body)
			written = append(written, carriedMailAccount{
				Signer:  strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer "),
				Written: body,
			})
			writer.WriteHeader(http.StatusOK)
		default:
			_, _ = writer.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(server.Close)
	return server, &written
}

func newMailCarryTestService(t *testing.T, planeURL string) *Service {
	t.Helper()
	configuration := Configuration{StateDirectory: t.TempDir()}
	if planeURL != "" {
		configuration.CentralPlaneAppURL = planeURL
		configuration.CentralPlaneProjectURL = planeURL
		configuration.CentralPlanePublishableKey = "publishable"
		configuration.CentralPlaneAgentKeyPath = writeAgentKeyForTest(t, "agent-key")
	}
	return NewService(configuration)
}

func seedRetiredMailAccounts(t *testing.T, databasePath string, actorEmails ...string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(databasePath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(databasePath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(context.Background(), `
CREATE TABLE IF NOT EXISTS mail_accounts (
	actor_email TEXT PRIMARY KEY, email TEXT NOT NULL, from_address TEXT NOT NULL,
	display_name TEXT NOT NULL, imap_host TEXT NOT NULL, imap_port INTEGER NOT NULL,
	imap_security TEXT NOT NULL, imap_username TEXT NOT NULL, imap_password TEXT NOT NULL,
	smtp_host TEXT NOT NULL, smtp_port INTEGER NOT NULL, smtp_security TEXT NOT NULL,
	smtp_username TEXT NOT NULL, smtp_password TEXT NOT NULL, default_mailbox TEXT NOT NULL,
	sent_mailbox TEXT NOT NULL, updated_at TEXT NOT NULL)`); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, actorEmail := range actorEmails {
		if _, errorValue := database.ExecContext(context.Background(), `
INSERT INTO mail_accounts VALUES (?, ?, ?, '', 'imap.example.com', 993, 'tls', ?, 'imap-secret',
	'smtp.example.com', 587, 'starttls', ?, 'smtp-secret', 'INBOX', 'Sent', '2026-01-01T00:00:00Z')`,
			actorEmail, actorEmail, actorEmail, actorEmail, actorEmail); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func TestMailAccountCarryWritesEachAccountAsThePersonWhoseItIs(t *testing.T) {
	plane, written := startPlaneKnowing(t, []string{"one@example.com", "two@example.com"})
	service := newMailCarryTestService(t, plane.URL)
	seedRetiredMailAccounts(t, service.stateDatabasePath(), "one@example.com", "two@example.com")

	report, errorValue := service.carryMailAccountsIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Accounts != 2 || len(report.Refused) != 0 {
		t.Fatalf("carry report = %#v", report)
	}
	signers := []string{}
	for _, carried := range *written {
		signers = append(signers, carried.Signer)
		if carried.Written["imapPassword"] != "imap-secret" {
			t.Fatalf("the password the device held was not carried: %#v", carried.Written)
		}
	}
	if !strings.Contains(strings.Join(signers, ","), "token-one@example.com") ||
		!strings.Contains(strings.Join(signers, ","), "token-two@example.com") {
		t.Fatalf("each account is written as the person whose it is: %v", signers)
	}
}

func TestMailAccountCarryRefusesAnActorTheCompanyDoesNotKnow(t *testing.T) {
	plane, written := startPlaneKnowing(t, []string{"one@example.com"})
	service := newMailCarryTestService(t, plane.URL)
	seedRetiredMailAccounts(t, service.stateDatabasePath(), "one@example.com", "stranger@example.com")

	report, errorValue := service.carryMailAccountsIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Accounts != 1 || len(report.Refused) != 1 {
		t.Fatalf("an account nobody here owns is named, not written: %#v", report)
	}
	if !strings.Contains(report.Refused[0], "stranger@example.com") {
		t.Fatalf("the refusal does not name the address: %q", report.Refused[0])
	}
	if len(*written) != 1 {
		t.Fatalf("a refused account was written anyway: %#v", *written)
	}
}

func TestMailAccountSweepDropsTheTableOnceTheCarryHasFinished(t *testing.T) {
	plane, _ := startPlaneKnowing(t, []string{"one@example.com"})
	service := newMailCarryTestService(t, plane.URL)
	seedRetiredMailAccounts(t, service.stateDatabasePath(), "one@example.com")

	service.sweepTheMailAccountsTheRecordNowHolds(context.Background())

	database, errorValue := service.openMailDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countCarriedMailTables(t, database, "mail_accounts") != 0 {
		t.Fatal("the accounts the record now holds survived the sweep")
	}
	if countCarriedMailTables(t, database, "mail_message_cache") != 1 {
		t.Fatal("a cache of what the mail server holds is not the record's to take")
	}
}

func TestMailAccountSweepKeepsAnAccountTheRecordRefused(t *testing.T) {
	plane, _ := startPlaneKnowing(t, []string{})
	service := newMailCarryTestService(t, plane.URL)
	seedRetiredMailAccounts(t, service.stateDatabasePath(), "stranger@example.com")

	service.sweepTheMailAccountsTheRecordNowHolds(context.Background())

	database, errorValue := service.openMailDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countCarriedMailTables(t, database, "mail_accounts") != 1 {
		t.Fatal("an account the record would not take was dropped anyway")
	}
}

func TestMailAccountSweepKeepsEverythingWhenNoCompanyIsNamed(t *testing.T) {
	service := newMailCarryTestService(t, "")
	seedRetiredMailAccounts(t, service.stateDatabasePath(), "one@example.com")

	service.sweepTheMailAccountsTheRecordNowHolds(context.Background())

	database, errorValue := service.openMailDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countCarriedMailTables(t, database, "mail_accounts") != 1 {
		t.Fatal("a device the record has never heard of dropped somebody's mail password")
	}
}

func countCarriedMailTables(t *testing.T, database *sql.DB, tableName string) int {
	t.Helper()
	var count int
	if errorValue := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	return count
}
