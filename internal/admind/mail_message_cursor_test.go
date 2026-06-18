package admind

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

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
