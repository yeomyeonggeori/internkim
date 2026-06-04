package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2"
)

type fakeMailBackend struct {
	testedAccount  mailAccount
	listedAccount  mailAccount
	listInput      mailMessageListRequest
	sentAccount    mailAccount
	sentMessage    mailMessageSendRequest
	movedUID       uint32
	movedTarget    string
	markedUID      uint32
	markedSeen     *bool
	markedFlagged  *bool
	readUID        uint32
	mailboxes      []mailMailboxResponse
	messages       []mailMessageResponse
	messageDetail  mailMessageDetailResponse
	shouldTestFail bool
	shouldListFail bool
	shouldSendFail bool
	shouldMoveFail bool
	shouldMarkFail bool
	shouldReadFail bool
}

func (backend *fakeMailBackend) TestAccount(ctx context.Context, account mailAccount) error {
	backend.testedAccount = account
	return nil
}

func (backend *fakeMailBackend) ListMailboxes(ctx context.Context, account mailAccount) ([]mailMailboxResponse, error) {
	backend.listedAccount = account
	return backend.mailboxes, nil
}

func (backend *fakeMailBackend) ListMessages(ctx context.Context, account mailAccount, input mailMessageListRequest) (mailMessageListResponse, error) {
	backend.listInput = input
	uids := mailMessageResponseUIDs(backend.messages)
	visibleUIDs, hasMoreMessages := visibleMailMessageUIDs(uids, input.Limit)
	messages := backend.messages
	if hasMoreMessages {
		messages = messages[:input.Limit]
	}
	return mailMessageListResponse{
		Messages:   messages,
		NextCursor: nextMailMessageCursor(input, visibleUIDs, hasMoreMessages),
	}, nil
}

func (backend *fakeMailBackend) ReadMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32) (mailMessageDetailResponse, error) {
	backend.readUID = uid
	return backend.messageDetail, nil
}

func (backend *fakeMailBackend) SendMessage(ctx context.Context, account mailAccount, input mailMessageSendRequest) (mailSendResult, error) {
	backend.sentAccount = account
	backend.sentMessage = input
	return mailSendResult{Sent: true, AppendedTo: account.SentMailbox}, nil
}

func (backend *fakeMailBackend) MoveMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32, targetMailbox string) error {
	backend.movedUID = uid
	backend.movedTarget = targetMailbox
	return nil
}

func (backend *fakeMailBackend) MarkMessage(ctx context.Context, account mailAccount, mailbox string, uid uint32, input mailMessageMarkRequest) error {
	backend.markedUID = uid
	backend.markedSeen = input.Seen
	backend.markedFlagged = input.Flagged
	return nil
}

func mailMessageResponseUIDs(messages []mailMessageResponse) []imap.UID {
	uids := make([]imap.UID, 0, len(messages))
	for _, message := range messages {
		uids = append(uids, imap.UID(message.UID))
	}
	return uids
}

func TestMailAccountSavePreservesStoredPasswords(t *testing.T) {
	service := newMailTestService(t)
	account := defaultMailAccount("admin@example.com")
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
	var account mailAccountResponse
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

func TestMailHandlersUseBackendForActions(t *testing.T) {
	service := newMailTestService(t)
	backend := &fakeMailBackend{
		mailboxes: []mailMailboxResponse{{Name: "INBOX", DisplayName: "Inbox", Total: 1}},
		messages:  []mailMessageResponse{{UID: 42, Mailbox: "INBOX", Subject: "Demo", From: "a@example.com"}},
		messageDetail: mailMessageDetailResponse{
			UID:     42,
			Mailbox: "INBOX",
			Subject: "Demo",
			From:    "a@example.com",
			Body:    "Body",
		},
	}
	service.mailBackend = backend
	saveConfiguredMailTestAccount(t, service)

	mailboxesResponse := performMailRequest(t, service, http.MethodGet, "/mail/api/mailboxes", "")
	if mailboxesResponse.Code != http.StatusOK {
		t.Fatalf("mailboxes status = %d body = %s", mailboxesResponse.Code, mailboxesResponse.Body.String())
	}
	messagesResponse := performMailRequest(t, service, http.MethodGet, "/mail/api/messages?mailbox=INBOX", "")
	if messagesResponse.Code != http.StatusOK {
		t.Fatalf("messages status = %d body = %s", messagesResponse.Code, messagesResponse.Body.String())
	}
	messageResponse := performMailRequest(t, service, http.MethodGet, "/mail/api/messages/INBOX/42", "")
	if messageResponse.Code != http.StatusOK || backend.readUID != 42 {
		t.Fatalf("message status = %d readUID = %d", messageResponse.Code, backend.readUID)
	}
	sendResponse := performMailRequest(t, service, http.MethodPost, "/mail/api/messages/send", `{"to":["recipient@example.com"],"subject":"Demo","body":"Hello"}`)
	if sendResponse.Code != http.StatusOK {
		t.Fatalf("send status = %d body = %s", sendResponse.Code, sendResponse.Body.String())
	}
	if !strings.Contains(backend.sentMessage.To[0], "recipient@example.com") || backend.sentAccount.ActorEmail != "admin@example.com" {
		t.Fatalf("sent message = %#v account = %#v", backend.sentMessage, backend.sentAccount)
	}
	moveResponse := performMailRequest(t, service, http.MethodPost, "/mail/api/messages/INBOX/42/move", `{"targetMailbox":"Archive"}`)
	if moveResponse.Code != http.StatusOK || backend.movedUID != 42 || backend.movedTarget != "Archive" {
		t.Fatalf("move status=%d uid=%d target=%q", moveResponse.Code, backend.movedUID, backend.movedTarget)
	}
	slashMailboxResponse := performMailRequest(t, service, http.MethodPost, "/mail/api/messages/%5BGmail%5D%2FAll%20Mail/42/move", `{"targetMailbox":"Archive"}`)
	if slashMailboxResponse.Code != http.StatusOK {
		t.Fatalf("slash mailbox move status=%d body=%s", slashMailboxResponse.Code, slashMailboxResponse.Body.String())
	}
	markResponse := performMailRequest(t, service, http.MethodPost, "/mail/api/messages/INBOX/42/flags", `{"seen":true}`)
	if markResponse.Code != http.StatusOK || backend.markedSeen == nil || !*backend.markedSeen {
		t.Fatalf("mark status=%d seen=%v", markResponse.Code, backend.markedSeen)
	}
}

func TestMailAccountRequiresAuthenticatedActor(t *testing.T) {
	service := newMailTestService(t)
	saveConfiguredMailTestAccount(t, service)

	request := httptest.NewRequest(http.MethodGet, "/mail/api/account", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	service.handleMail(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestMailAccountDoesNotFallbackToAdminForAuthenticatedUser(t *testing.T) {
	service := newMailTestService(t)
	saveConfiguredMailTestAccount(t, service)

	accountResponse := performMailRequestAs(t, service, "staff@example.com", http.MethodGet, "/mail/api/account", "")
	if accountResponse.Code != http.StatusOK {
		t.Fatalf("account status = %d body = %s", accountResponse.Code, accountResponse.Body.String())
	}
	var account mailAccountResponse
	if errorValue := json.Unmarshal(accountResponse.Body.Bytes(), &account); errorValue != nil {
		t.Fatal(errorValue)
	}
	if account.IsConfigured || account.Email != "staff@example.com" {
		t.Fatalf("account should be an unconfigured staff account: %#v", account)
	}

	mailboxesResponse := performMailRequestAs(t, service, "staff@example.com", http.MethodGet, "/mail/api/mailboxes", "")
	if mailboxesResponse.Code != http.StatusBadRequest {
		t.Fatalf("mailboxes status = %d body = %s", mailboxesResponse.Code, mailboxesResponse.Body.String())
	}
}

func TestMailRequiresConfiguredAccountForMailboxReads(t *testing.T) {
	service := newMailTestService(t)
	response := performMailRequest(t, service, http.MethodGet, "/mail/api/mailboxes", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	accountResponse := performMailRequest(t, service, http.MethodGet, "/mail/api/account", "")
	if accountResponse.Code != http.StatusOK {
		t.Fatalf("account status = %d", accountResponse.Code)
	}
	var account mailAccountResponse
	if errorValue := json.Unmarshal(accountResponse.Body.Bytes(), &account); errorValue != nil {
		t.Fatal(errorValue)
	}
	if account.IsConfigured {
		t.Fatalf("new account should not be configured: %#v", account)
	}
}

func TestMailMessagesReturnNextCursor(t *testing.T) {
	service := newMailTestService(t)
	backend := &fakeMailBackend{
		messages: []mailMessageResponse{
			{UID: 12, Mailbox: "INBOX", Subject: "Newest"},
			{UID: 11, Mailbox: "INBOX", Subject: "Older"},
			{UID: 10, Mailbox: "INBOX", Subject: "Oldest"},
		},
	}
	service.mailBackend = backend
	saveConfiguredMailTestAccount(t, service)

	response := performMailRequest(t, service, http.MethodGet, "/mail/api/messages?mailbox=INBOX&limit=2", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var result mailMessageListResponse
	if errorValue := json.Unmarshal(response.Body.Bytes(), &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(result.Messages) != 2 || result.NextCursor == "" {
		t.Fatalf("result = %#v", result)
	}
	cursor, errorValue := decodeMailMessageCursor(result.NextCursor)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if cursor.Mailbox != "INBOX" || cursor.BeforeUID != 11 {
		t.Fatalf("cursor = %#v", cursor)
	}
}

func TestMailMessagesApplyCursorToRequest(t *testing.T) {
	service := newMailTestService(t)
	backend := &fakeMailBackend{}
	service.mailBackend = backend
	saveConfiguredMailTestAccount(t, service)
	cursor := encodeMailMessageCursor("INBOX", "invoice", 20)

	response := performMailRequest(t, service, http.MethodGet, "/mail/api/messages?mailbox=INBOX&query=invoice&limit=5&cursor="+cursor, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if backend.listInput.BeforeUID != 20 || backend.listInput.Query != "invoice" || backend.listInput.Limit != 5 {
		t.Fatalf("list input = %#v", backend.listInput)
	}
}

func TestMailMessagesRejectInvalidCursor(t *testing.T) {
	service := newMailTestService(t)
	service.mailBackend = &fakeMailBackend{}
	saveConfiguredMailTestAccount(t, service)

	response := performMailRequest(t, service, http.MethodGet, "/mail/api/messages?mailbox=INBOX&cursor=not-a-cursor", "")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid cursor") {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestMailMessagesRejectMismatchedCursor(t *testing.T) {
	service := newMailTestService(t)
	service.mailBackend = &fakeMailBackend{}
	saveConfiguredMailTestAccount(t, service)
	cursor := encodeMailMessageCursor("Sent", "", 20)

	response := performMailRequest(t, service, http.MethodGet, "/mail/api/messages?mailbox=INBOX&cursor="+cursor, "")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid cursor") {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestMailMessagePageUIDsReturnNewestFirst(t *testing.T) {
	uids := mailMessagePageUIDs([]imap.UID{7, 9, 8}, 2)
	if len(uids) != 2 || uids[0] != 9 || uids[1] != 8 {
		t.Fatalf("uids = %#v", uids)
	}
}

func TestMailMailboxResponsesFromListDataSkipsNonSelectableMailboxes(t *testing.T) {
	total := uint32(3)
	unseen := uint32(1)
	mailboxes := mailMailboxResponsesFromListData([]*imap.ListData{
		{Mailbox: "INBOX", Status: &imap.StatusData{NumMessages: &total, NumUnseen: &unseen}},
		{Mailbox: "Folders", Attrs: []imap.MailboxAttr{imap.MailboxAttrNoSelect}},
	})
	if len(mailboxes) != 1 {
		t.Fatalf("mailboxes = %#v", mailboxes)
	}
	if mailboxes[0].Name != "INBOX" || mailboxes[0].Total != 3 || mailboxes[0].Unseen != 1 {
		t.Fatalf("mailbox = %#v", mailboxes[0])
	}
}

func TestDecodeMailHeaderDecodesEncodedWords(t *testing.T) {
	encodedSubject := mime.QEncoding.Encode("utf-8", "테스트 제목")
	if subject := decodeMailHeader(encodedSubject); subject != "테스트 제목" {
		t.Fatalf("subject = %q", subject)
	}
}

func TestIMAPAddressListStringDecodesDisplayNames(t *testing.T) {
	encodedName := mime.QEncoding.Encode("utf-8", "네이버")
	addresses := []imap.Address{{Name: encodedName, Mailbox: "account_noreply", Host: "navercorp.com"}}
	if value := imapAddressListString(addresses); value != "네이버 <account_noreply@navercorp.com>" {
		t.Fatalf("address = %q", value)
	}
}

func TestParseMailDocumentKeepsHTMLBody(t *testing.T) {
	document := strings.Join([]string{
		"Content-Type: multipart/alternative; boundary=frontier",
		"",
		"--frontier",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Plain body",
		"--frontier",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<html><body><strong>HTML body</strong></body></html>",
		"--frontier--",
		"",
	}, "\r\n")
	parsedDocument := parseMailDocument([]byte(document))
	if parsedDocument.PlainText != "Plain body" {
		t.Fatalf("plain text = %q", parsedDocument.PlainText)
	}
	if !strings.Contains(parsedDocument.HTML, "<strong>HTML body</strong>") {
		t.Fatalf("html = %q", parsedDocument.HTML)
	}
}

func TestParseMailDocumentFallsBackToHTMLText(t *testing.T) {
	document := strings.Join([]string{
		"Content-Type: text/html; charset=utf-8",
		"",
		"<html><body><strong>HTML only</strong></body></html>",
	}, "\r\n")
	parsedDocument := parseMailDocument([]byte(document))
	if parsedDocument.PlainText != "HTML only" {
		t.Fatalf("plain text = %q", parsedDocument.PlainText)
	}
	if !strings.Contains(parsedDocument.HTML, "<strong>HTML only</strong>") {
		t.Fatalf("html = %q", parsedDocument.HTML)
	}
}

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
