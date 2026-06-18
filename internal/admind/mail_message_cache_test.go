package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCachedMailMessagesRequireMatchingCursor(t *testing.T) {
	service := newMailTestService(t)
	saveConfiguredMailTestAccount(t, service)
	if errorValue := service.saveCachedMailMessages(context.Background(), "admin@example.com", mailMessageListRequest{
		Mailbox: "INBOX",
		Limit:   15,
	}, mailMessageListResponse{Messages: []mailMessageResponse{{UID: 42, Mailbox: "INBOX", Subject: "First page"}}}); errorValue != nil {
		t.Fatal(errorValue)
	}

	_, found, errorValue := service.readCachedMailMessages(context.Background(), "admin@example.com", mailMessageListRequest{
		Mailbox:   "INBOX",
		Limit:     15,
		BeforeUID: 42,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found {
		t.Fatal("cursor page used first page cache marker")
	}
}

func TestCachedMailMessagesRequireMatchingLimit(t *testing.T) {
	service := newMailTestService(t)
	saveConfiguredMailTestAccount(t, service)
	if errorValue := service.saveCachedMailMessages(context.Background(), "admin@example.com", mailMessageListRequest{
		Mailbox: "INBOX",
		Limit:   15,
	}, mailMessageListResponse{Messages: []mailMessageResponse{{UID: 42, Mailbox: "INBOX", Subject: "First page"}}}); errorValue != nil {
		t.Fatal(errorValue)
	}

	_, found, errorValue := service.readCachedMailMessages(context.Background(), "admin@example.com", mailMessageListRequest{
		Mailbox: "INBOX",
		Limit:   10,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found {
		t.Fatal("cached page ignored requested limit")
	}
}

func TestCachedMailMessagesKeepSavedNextCursor(t *testing.T) {
	service := newMailTestService(t)
	saveConfiguredMailTestAccount(t, service)
	nextCursor := encodeMailMessageCursor("INBOX", "", 12)
	if errorValue := service.saveCachedMailMessages(context.Background(), "admin@example.com", mailMessageListRequest{
		Mailbox: "INBOX",
		Limit:   15,
	}, mailMessageListResponse{
		Messages:   []mailMessageResponse{{UID: 42, Mailbox: "INBOX", Subject: "First page"}},
		NextCursor: nextCursor,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	result, found, errorValue := service.readCachedMailMessages(context.Background(), "admin@example.com", mailMessageListRequest{
		Mailbox: "INBOX",
		Limit:   15,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("cached page not found")
	}
	if result.NextCursor != nextCursor {
		t.Fatalf("next cursor = %q want %q", result.NextCursor, nextCursor)
	}
}

func TestCachedMailMessagesReplacePageSnapshot(t *testing.T) {
	service := newMailTestService(t)
	saveConfiguredMailTestAccount(t, service)
	input := mailMessageListRequest{
		Mailbox: "INBOX",
		Limit:   15,
	}
	if errorValue := service.saveCachedMailMessages(context.Background(), "admin@example.com", input, mailMessageListResponse{
		Messages: []mailMessageResponse{
			{UID: 42, Mailbox: "INBOX", Subject: "Old newest"},
			{UID: 41, Mailbox: "INBOX", Subject: "Old older"},
		},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.saveCachedMailMessages(context.Background(), "admin@example.com", input, mailMessageListResponse{
		Messages: []mailMessageResponse{{UID: 43, Mailbox: "INBOX", Subject: "Current newest"}},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	result, found, errorValue := service.readCachedMailMessages(context.Background(), "admin@example.com", input)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("cached page not found")
	}
	if len(result.Messages) != 1 || result.Messages[0].UID != 43 {
		t.Fatalf("messages = %#v", result.Messages)
	}
}

func TestMailMessageListIgnoresCacheWriteFailure(t *testing.T) {
	service := newMailTestService(t)
	backend := &fakeMailBackend{
		messages: []mailMessageResponse{{UID: 42, Mailbox: "INBOX", Subject: "Remote"}},
	}
	service.mailBackend = backend
	saveConfiguredMailTestAccount(t, service)
	dropMailCacheTable(t, service, "mail_message_cache")

	response := performMailRequest(t, service, http.MethodGet, "/mail/api/messages?mailbox=INBOX", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var result mailMessageListResponse
	if errorValue := json.Unmarshal(response.Body.Bytes(), &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(result.Messages) != 1 || result.Messages[0].UID != 42 {
		t.Fatalf("messages = %#v", result.Messages)
	}
}

func TestMailMessageMoveIgnoresCacheDeleteFailure(t *testing.T) {
	service := newMailTestService(t)
	backend := &fakeMailBackend{}
	service.mailBackend = backend
	saveConfiguredMailTestAccount(t, service)
	dropMailCacheTable(t, service, "mail_message_cache")

	response := performMailRequest(t, service, http.MethodPost, "/mail/api/messages/INBOX/42/move", `{"targetMailbox":"Archive"}`)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if backend.movedUID != 42 || backend.movedTarget != "Archive" {
		t.Fatalf("move uid = %d target = %q", backend.movedUID, backend.movedTarget)
	}
}

func dropMailCacheTable(t *testing.T, service *Service, tableName string) {
	t.Helper()
	database, errorValue := service.openMailDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+tableName); errorValue != nil {
		t.Fatal(errorValue)
	}
}
