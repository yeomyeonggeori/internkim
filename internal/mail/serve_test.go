package mail

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recordingBackend struct {
	asked    []string
	accounts []Account
	list     MessageListRequest
	send     MessageSendRequest
	mark     MessageMarkRequest
	mailbox  string
	uid      uint32
	target   string
	refuse   error
}

func (backend *recordingBackend) note(capability string, account Account) {
	backend.asked = append(backend.asked, capability)
	backend.accounts = append(backend.accounts, account)
}

func (backend *recordingBackend) TestAccount(_ context.Context, account Account) error {
	backend.note("test", account)
	return backend.refuse
}

func (backend *recordingBackend) ListMailboxes(_ context.Context, account Account) ([]MailboxResponse, error) {
	backend.note("mailboxes", account)
	return []MailboxResponse{{Name: "INBOX"}}, backend.refuse
}

func (backend *recordingBackend) ListMessages(_ context.Context, account Account, input MessageListRequest) (MessageListResponse, error) {
	backend.note("messages", account)
	backend.list = input
	return MessageListResponse{Messages: []MessageResponse{{UID: 7, Subject: "회의"}}}, backend.refuse
}

func (backend *recordingBackend) ReadMessage(_ context.Context, account Account, mailbox string, uid uint32) (MessageDetailResponse, error) {
	backend.note("message", account)
	backend.mailbox, backend.uid = mailbox, uid
	return MessageDetailResponse{UID: uid, Mailbox: mailbox}, backend.refuse
}

func (backend *recordingBackend) SendMessage(_ context.Context, account Account, input MessageSendRequest) (SendResult, error) {
	backend.note("send", account)
	backend.send = input
	return SendResult{}, backend.refuse
}

func (backend *recordingBackend) MoveMessage(_ context.Context, account Account, mailbox string, uid uint32, targetMailbox string) error {
	backend.note("move", account)
	backend.mailbox, backend.uid, backend.target = mailbox, uid, targetMailbox
	return backend.refuse
}

func (backend *recordingBackend) MarkMessage(_ context.Context, account Account, mailbox string, uid uint32, input MessageMarkRequest) error {
	backend.note("mark", account)
	backend.mailbox, backend.uid, backend.mark = mailbox, uid, input
	return backend.refuse
}

func aConfiguredAccount() Account {
	return Account{
		ActorEmail:   "first@example.test",
		Email:        "first@example.test",
		IMAPHost:     "imap.example.test",
		IMAPUsername: "first",
		IMAPPassword: "secret",
		SMTPHost:     "smtp.example.test",
		SMTPUsername: "first",
		SMTPPassword: "secret",
	}
}

func ask(t *testing.T, backend Backend, capability string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload, errorValue := json.Marshal(body)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, servePrefix+capability, strings.NewReader(string(payload)))
	recorder := httptest.NewRecorder()
	Handler(backend).ServeHTTP(recorder, request)
	return recorder
}

func TestHandlerCarriesTheCallersOwnAccountToTheBackend(t *testing.T) {
	backend := &recordingBackend{}
	account := aConfiguredAccount()

	recorder := ask(t, backend, "mailboxes", map[string]any{"account": account})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if len(backend.accounts) != 1 || backend.accounts[0].IMAPUsername != "first" {
		t.Fatalf("accounts = %+v", backend.accounts)
	}
}

func TestHandlerRefusesACallWithNoAccountBeforeReachingTheMessenger(t *testing.T) {
	backend := &recordingBackend{}

	recorder := ask(t, backend, "messages", map[string]any{"account": Account{Email: "first@example.test"}})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	if len(backend.asked) != 0 {
		t.Fatalf("a half-written account still reached the backend: %v", backend.asked)
	}
}

func TestHandlerPassesEachOperationItsOwnArguments(t *testing.T) {
	backend := &recordingBackend{}
	account := aConfiguredAccount()

	ask(t, backend, "messages", map[string]any{
		"account": account,
		"list":    map[string]any{"mailbox": "INBOX", "limit": 25, "beforeUID": 90},
	})
	ask(t, backend, "move", map[string]any{
		"account": account, "mailbox": "INBOX", "uid": 7, "targetMailbox": "Archive",
	})
	ask(t, backend, "mark", map[string]any{
		"account": account, "mailbox": "INBOX", "uid": 7, "mark": map[string]any{"seen": true},
	})

	if backend.list.Mailbox != "INBOX" || backend.list.Limit != 25 || backend.list.BeforeUID != 90 {
		t.Fatalf("list = %+v", backend.list)
	}
	if backend.target != "Archive" || backend.uid != 7 {
		t.Fatalf("move and mark = %q %d", backend.target, backend.uid)
	}
	if backend.mark.Seen == nil || !*backend.mark.Seen {
		t.Fatalf("mark = %+v", backend.mark)
	}
}

func TestHandlerReportsAMessengerRefusalWithoutPretendingItWorked(t *testing.T) {
	backend := &recordingBackend{refuse: errors.New("imap said no")}

	recorder := ask(t, backend, "mailboxes", map[string]any{"account": aConfiguredAccount()})

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "imap said no") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestHandlerAnswersNothingItDoesNotKnow(t *testing.T) {
	backend := &recordingBackend{}

	if recorder := ask(t, backend, "delete-everything", map[string]any{"account": aConfiguredAccount()}); recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
	request := httptest.NewRequest(http.MethodGet, servePrefix+"mailboxes", nil)
	recorder := httptest.NewRecorder()
	Handler(backend).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("a read of a mail capability = %d", recorder.Code)
	}
	if len(backend.asked) != 0 {
		t.Fatalf("backend was reached: %v", backend.asked)
	}
}
