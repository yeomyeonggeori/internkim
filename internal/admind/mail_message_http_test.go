package admind

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

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
