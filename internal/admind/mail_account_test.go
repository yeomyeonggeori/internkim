package admind

import (
	"context"
	"encoding/json"
	"github.com/yeomyeonggeori/internkim/internal/mail"
	"net/http"
	"testing"
)

func TestMailAccountSavePreservesStoredPasswords(t *testing.T) {
	service := newMailTestService(t)
	account := mail.DefaultAccount("admin@example.com")
	account.IMAPHost = "imap.example.com"
	account.IMAPUsername = "admin@example.com"
	account.IMAPPassword = "imap-secret"
	account.SMTPHost = "smtp.example.com"
	account.SMTPUsername = "admin@example.com"
	account.SMTPPassword = "smtp-secret"
	if errorValue := service.saveMailAccountRecord(context.Background(), account); errorValue != nil {
		t.Fatal(errorValue)
	}

	response := performMailRequest(t, service, http.MethodPut, "/mail/api/account", `{
		"email":"admin@example.com",
		"fromAddress":"Admin <admin@example.com>",
		"imapHost":"imap.changed.example.com",
		"imapPort":993,
		"imapSecurity":"tls",
		"imapUsername":"admin@example.com",
		"smtpHost":"smtp.changed.example.com",
		"smtpPort":587,
		"smtpSecurity":"starttls",
		"smtpUsername":"admin@example.com",
		"defaultMailbox":"INBOX",
		"sentMailbox":"Sent"
	}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	savedAccount, found, errorValue := service.readMailAccount(context.Background(), "admin@example.com")
	if errorValue != nil || !found {
		t.Fatalf("read account found=%v error=%v", found, errorValue)
	}
	if savedAccount.IMAPPassword != "imap-secret" || savedAccount.SMTPPassword != "smtp-secret" {
		t.Fatalf("passwords were not preserved: %#v", savedAccount)
	}
	if savedAccount.IMAPHost != "imap.changed.example.com" || savedAccount.SMTPHost != "smtp.changed.example.com" {
		t.Fatalf("hosts were not saved: %#v", savedAccount)
	}
}

func TestMailAccountSavePersistsForSubsequentRequests(t *testing.T) {
	service := newMailTestService(t)

	saveResponse := performMailRequest(t, service, http.MethodPut, "/mail/api/account", `{
		"email":"admin@example.com",
		"fromAddress":"Admin <admin@example.com>",
		"displayName":"Admin",
		"imapHost":"imap.example.com",
		"imapPort":993,
		"imapSecurity":"tls",
		"imapUsername":"admin@example.com",
		"imapPassword":"imap-secret",
		"smtpHost":"smtp.example.com",
		"smtpPort":587,
		"smtpSecurity":"starttls",
		"smtpUsername":"admin@example.com",
		"smtpPassword":"smtp-secret",
		"defaultMailbox":"INBOX",
		"sentMailbox":"Sent"
	}`)
	if saveResponse.Code != http.StatusOK {
		t.Fatalf("save status = %d body = %s", saveResponse.Code, saveResponse.Body.String())
	}

	accountResponse := performMailRequest(t, service, http.MethodGet, "/mail/api/account", "")
	if accountResponse.Code != http.StatusOK {
		t.Fatalf("account status = %d body = %s", accountResponse.Code, accountResponse.Body.String())
	}
	var account mail.AccountResponse
	if errorValue := json.Unmarshal(accountResponse.Body.Bytes(), &account); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !account.IsConfigured || account.Email != "admin@example.com" || account.IMAPHost != "imap.example.com" || account.SMTPHost != "smtp.example.com" {
		t.Fatalf("account was not persisted for later page loads: %#v", account)
	}
	if !account.HasIMAPPassword || !account.HasSMTPPassword {
		t.Fatalf("password presence was not persisted: %#v", account)
	}
}

func TestMailAccountSaveAllowsEmptySentMailbox(t *testing.T) {
	service := newMailTestService(t)

	saveResponse := performMailRequest(t, service, http.MethodPut, "/mail/api/account", `{
		"email":"admin@example.com",
		"fromAddress":"Admin <admin@example.com>",
		"displayName":"Admin",
		"imapHost":"imap.gmail.com",
		"imapPort":993,
		"imapSecurity":"tls",
		"imapUsername":"admin@example.com",
		"imapPassword":"imap-secret",
		"smtpHost":"smtp.gmail.com",
		"smtpPort":587,
		"smtpSecurity":"starttls",
		"smtpUsername":"admin@example.com",
		"smtpPassword":"smtp-secret",
		"defaultMailbox":"INBOX",
		"sentMailbox":""
	}`)
	if saveResponse.Code != http.StatusOK {
		t.Fatalf("save status = %d body = %s", saveResponse.Code, saveResponse.Body.String())
	}

	savedAccount, found, errorValue := service.readMailAccount(context.Background(), "admin@example.com")
	if errorValue != nil || !found {
		t.Fatalf("read account found=%v error=%v", found, errorValue)
	}
	if savedAccount.SentMailbox != "" {
		t.Fatalf("sent mailbox = %q", savedAccount.SentMailbox)
	}
}
