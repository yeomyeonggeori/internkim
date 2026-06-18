package admind

import (
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
