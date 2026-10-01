package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/mail"
)

func newMailTestService(t *testing.T) *Service {
	t.Helper()
	return newMailTestServiceOn(t, startPlaneHoldingMailAccounts(t))
}

func newMailTestServiceOn(t *testing.T, plane *httptest.Server) *Service {
	t.Helper()
	stateDirectory := t.TempDir()
	return NewService(Configuration{
		StateDirectory:             stateDirectory,
		TaskDatabasePath:           filepath.Join(stateDirectory, "flow.sqlite"),
		MailDatabasePath:           filepath.Join(stateDirectory, "mail.sqlite"),
		AdminEmailPath:             writeTestFile(t, "admin@example.com"),
		UsersSyncStatePath:         filepath.Join(stateDirectory, "users-sync.json"),
		CentralPlaneAppURL:         plane.URL,
		CentralPlaneProjectURL:     plane.URL,
		CentralPlanePublishableKey: "publishable",
		CentralPlaneAgentKeyPath:   writeAgentKeyForTest(t, "agent-key"),
	})
}

func writeUsersSyncTestCache(t *testing.T, service *Service, email string) {
	t.Helper()
	stateDirectory := filepath.Dir(service.Configuration.UsersSyncStatePath)
	if errorValue := os.MkdirAll(stateDirectory, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(service.Configuration.UsersSyncStatePath, []byte(`{"users":["`+email+`"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func saveConfiguredMailTestAccount(t *testing.T, service *Service) {
	t.Helper()
	account := mail.DefaultAccount("admin@example.com")
	account.FromAddress = "admin@example.com"
	account.IMAPHost = "imap.example.com"
	account.IMAPUsername = "admin@example.com"
	account.IMAPPassword = "imap-secret"
	account.SMTPHost = "smtp.example.com"
	account.SMTPUsername = "admin@example.com"
	account.SMTPPassword = "smtp-secret"
	if errorValue := service.saveMailAccountRecord(context.Background(), account); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func performMailRequest(t *testing.T, service *Service, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	return performMailRequestAs(t, service, "admin@example.com", method, path, body)
}

func performMailRequestAs(t *testing.T, service *Service, actorEmail string, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	request := httptest.NewRequest(method, path, reader)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("CF-Access-Authenticated-User-Email", actorEmail)
	if strings.TrimSpace(body) != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	service.handleMail(response, request)
	return response
}

func startPlaneHoldingMailAccounts(t *testing.T) *httptest.Server {
	t.Helper()
	heldByEmail := map[string]map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/api/agent/session":
			var asked struct {
				ExternalID string `json:"externalID"`
			}
			_ = json.NewDecoder(request.Body).Decode(&asked)
			_, _ = writer.Write([]byte(`{"memberID":"member-` + asked.ExternalID + `","accessToken":"token-` + asked.ExternalID + `","expiresAt":99999999999}`))
		case request.URL.Path == "/api/agent/member":
			email := request.URL.Query().Get("email")
			_, _ = writer.Write([]byte(`{"member":{"memberID":"member-` + email + `","email":"` + email + `"}}`))
		case request.URL.Path == "/api/agent/mail-account":
			email := strings.TrimPrefix(request.URL.Query().Get("memberID"), "member-")
			writeHeldMailAccount(writer, heldByEmail[email])
		case request.URL.Path == "/api/agent/mail-accounts":
			configured := []map[string]any{}
			for _, held := range heldByEmail {
				if held["IMAPHost"] != "" {
					configured = append(configured, held)
				}
			}
			document, _ := json.Marshal(map[string]any{"accounts": configured})
			_, _ = writer.Write(document)
		case request.URL.Path == "/api/member/mail-account" && request.Method == http.MethodPut:
			email := strings.TrimPrefix(strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer "), "token-")
			var written map[string]any
			_ = json.NewDecoder(request.Body).Decode(&written)
			heldByEmail[email] = heldMailAccountOf(email, written)
			writer.WriteHeader(http.StatusOK)
		default:
			_, _ = writer.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func writeHeldMailAccount(writer http.ResponseWriter, held map[string]any) {
	if held == nil {
		_, _ = writer.Write([]byte(`{"account":null}`))
		return
	}
	document, _ := json.Marshal(map[string]any{"account": held})
	_, _ = writer.Write(document)
}

func heldMailAccountOf(actorEmail string, written map[string]any) map[string]any {
	text := func(field string) string {
		value, _ := written[field].(string)
		return value
	}
	port := func(field string) int {
		value, _ := written[field].(float64)
		return int(value)
	}
	return map[string]any{
		"ActorEmail": actorEmail, "Email": text("email"), "FromAddress": text("fromAddress"),
		"DisplayName": text("displayName"), "IMAPHost": text("imapHost"), "IMAPPort": port("imapPort"),
		"IMAPSecurity": text("imapSecurity"), "IMAPUsername": text("imapUsername"),
		"IMAPPassword": text("imapPassword"), "SMTPHost": text("smtpHost"), "SMTPPort": port("smtpPort"),
		"SMTPSecurity": text("smtpSecurity"), "SMTPUsername": text("smtpUsername"),
		"SMTPPassword": text("smtpPassword"), "DefaultMailbox": text("defaultMailbox"),
		"SentMailbox": text("sentMailbox"),
	}
}
