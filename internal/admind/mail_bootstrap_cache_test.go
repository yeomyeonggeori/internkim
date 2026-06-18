package admind

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestMailBootstrapReturnsCachedState(t *testing.T) {
	service := newMailTestService(t)
	service.mailBackend = &fakeMailBackend{
		mailboxes: []mailMailboxResponse{{Name: "INBOX", DisplayName: "Inbox", Total: 1}},
		messages:  []mailMessageResponse{{UID: 42, Mailbox: "INBOX", Subject: "Cached", From: "a@example.com"}},
	}
	saveConfiguredMailTestAccount(t, service)

	mailboxesResponse := performMailRequest(t, service, http.MethodGet, "/mail/api/mailboxes", "")
	if mailboxesResponse.Code != http.StatusOK {
		t.Fatalf("mailboxes status = %d body = %s", mailboxesResponse.Code, mailboxesResponse.Body.String())
	}
	messagesResponse := performMailRequest(t, service, http.MethodGet, "/mail/api/messages?mailbox=INBOX&limit=15", "")
	if messagesResponse.Code != http.StatusOK {
		t.Fatalf("messages status = %d body = %s", messagesResponse.Code, messagesResponse.Body.String())
	}

	response := performMailRequest(t, service, http.MethodGet, "/mail/api/bootstrap?mailbox=INBOX&limit=15", "")
	if response.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body = %s", response.Code, response.Body.String())
	}
	var result mailBootstrapResponse
	if errorValue := json.Unmarshal(response.Body.Bytes(), &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !result.HasCachedMailboxes || !result.HasCachedMessages {
		t.Fatalf("cache flags = mailboxes:%v messages:%v", result.HasCachedMailboxes, result.HasCachedMessages)
	}
	if len(result.Mailboxes) != 1 || result.Mailboxes[0].Name != "INBOX" {
		t.Fatalf("mailboxes = %#v", result.Mailboxes)
	}
	if len(result.Messages) != 1 || result.Messages[0].UID != 42 {
		t.Fatalf("messages = %#v", result.Messages)
	}
}

func TestMailBootstrapKeepsCachedNextCursor(t *testing.T) {
	service := newMailTestService(t)
	service.mailBackend = &fakeMailBackend{
		messages: mailMessageTestPage(16),
	}
	saveConfiguredMailTestAccount(t, service)

	messagesResponse := performMailRequest(t, service, http.MethodGet, "/mail/api/messages?mailbox=INBOX&limit=15", "")
	if messagesResponse.Code != http.StatusOK {
		t.Fatalf("messages status = %d body = %s", messagesResponse.Code, messagesResponse.Body.String())
	}
	var messagesResult mailMessageListResponse
	if errorValue := json.Unmarshal(messagesResponse.Body.Bytes(), &messagesResult); errorValue != nil {
		t.Fatal(errorValue)
	}
	if messagesResult.NextCursor == "" {
		t.Fatalf("live next cursor is empty: %#v", messagesResult)
	}

	response := performMailRequest(t, service, http.MethodGet, "/mail/api/bootstrap?mailbox=INBOX&limit=15", "")
	if response.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body = %s", response.Code, response.Body.String())
	}
	var result mailBootstrapResponse
	if errorValue := json.Unmarshal(response.Body.Bytes(), &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.NextCursor != messagesResult.NextCursor {
		t.Fatalf("cached next cursor = %q want %q", result.NextCursor, messagesResult.NextCursor)
	}
}

func TestMailBootstrapRemembersEmptyCachedMailbox(t *testing.T) {
	service := newMailTestService(t)
	service.mailBackend = &fakeMailBackend{
		mailboxes: []mailMailboxResponse{{Name: "INBOX", DisplayName: "Inbox", Total: 0}},
		messages:  []mailMessageResponse{},
	}
	saveConfiguredMailTestAccount(t, service)

	messagesResponse := performMailRequest(t, service, http.MethodGet, "/mail/api/messages?mailbox=INBOX&limit=15", "")
	if messagesResponse.Code != http.StatusOK {
		t.Fatalf("messages status = %d body = %s", messagesResponse.Code, messagesResponse.Body.String())
	}

	response := performMailRequest(t, service, http.MethodGet, "/mail/api/bootstrap?mailbox=INBOX&limit=15", "")
	if response.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body = %s", response.Code, response.Body.String())
	}
	var result mailBootstrapResponse
	if errorValue := json.Unmarshal(response.Body.Bytes(), &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !result.HasCachedMessages || len(result.Messages) != 0 {
		t.Fatalf("hasCachedMessages = %v messages = %#v", result.HasCachedMessages, result.Messages)
	}
}

func mailMessageTestPage(count int) []mailMessageResponse {
	messages := make([]mailMessageResponse, 0, count)
	for uid := count; uid >= 1; uid-- {
		messages = append(messages, mailMessageResponse{
			UID:     uint32(uid),
			Mailbox: "INBOX",
			Subject: "Message",
		})
	}
	return messages
}
