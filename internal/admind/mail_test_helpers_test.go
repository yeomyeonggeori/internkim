package admind

import (
	"bytes"
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func newMailTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(Configuration{
		StateDirectory:   t.TempDir(),
		MailDatabasePath: filepath.Join(t.TempDir(), "mail.sqlite"),
		AdminEmailPath:   writeTestFile(t, "admin@example.com"),
	})
}

func saveConfiguredMailTestAccount(t *testing.T, service *Service) {
	t.Helper()
	account := defaultMailAccount("admin@example.com")
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
