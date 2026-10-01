package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitlab.com/eastriver/internkim/internal/mail"
)

func startPlaneHoldingASealedMailAccount(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/agent/member":
			_, _ = writer.Write([]byte(`{"member":{"memberID":"member-looked-up","email":"admin@example.com"}}`))
		case "/api/agent/mail-account":
			_, _ = writer.Write([]byte(`{"account":{"MemberID":"member-the-record-claims","IMAPHost":"imap.example.com","SMTPHost":"smtp.example.com",` +
				`"IMAPPassword":"","SealedIMAPPassword":{"version":1,"recipient":"recipient","ephemeralPublicKey":"ephemeral","nonce":"nonce","ciphertext":"ciphertext"}}}`))
		default:
			_, _ = writer.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestAHeldMailPasswordIsOpenedForTheMemberThisComputerLookedUp(t *testing.T) {
	service := newMailTestServiceOn(t, startPlaneHoldingASealedMailAccount(t))
	var openedFor string
	service.mailPasswords = func(account mail.Account, memberID string) (mail.Account, error) {
		openedFor = memberID
		if account.SealedIMAPPassword == nil || account.SealedIMAPPassword.Ciphertext != "ciphertext" {
			t.Fatalf("the sealed password did not reach the opener: %#v", account.SealedIMAPPassword)
		}
		account.IMAPPassword, account.SealedIMAPPassword = "opened", nil
		return account, nil
	}

	account, found, errorValue := service.readMailAccount(context.Background(), "admin@example.com")

	if errorValue != nil || !found {
		t.Fatalf("found=%v error=%v", found, errorValue)
	}
	if openedFor != "member-looked-up" {
		t.Fatalf("opened for %q", openedFor)
	}
	if account.IMAPPassword != "opened" {
		t.Fatalf("password = %q", account.IMAPPassword)
	}
}

func TestAHeldMailPasswordThisComputerCannotOpenIsAnError(t *testing.T) {
	service := newMailTestServiceOn(t, startPlaneHoldingASealedMailAccount(t))
	service.mailPasswords = mail.BoxPasswords(t.TempDir())

	if _, _, errorValue := service.readMailAccount(context.Background(), "admin@example.com"); errorValue == nil {
		t.Fatal("a sealed password with no box to open it was read as an account")
	}
}
