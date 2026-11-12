package admind

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	nostr "github.com/nbd-wtf/go-nostr"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

func sentThroughChatd(t *testing.T, payload map[string]string) map[string]string {
	t.Helper()
	var sent map[string]string
	chatd := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		json.NewDecoder(request.Body).Decode(&sent)
		json.NewEncoder(writer).Encode(directoryDirectMessageResponse{ChannelID: "dm-uuid", MessageID: "event-1"})
	}))
	defer chatd.Close()
	service := buzzKeyTestService(t, "sample@example.com")
	directory := service.HTTPClient
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == chatd.Listener.Addr().String() {
			return http.DefaultTransport.RoundTrip(request)
		}
		return directory.Transport.RoundTrip(request)
	})}
	service.Configuration.ChatdEndpoint = chatd.URL
	service.Configuration.ChatdPlatform = "buzz"
	body, _ := json.Marshal(payload)
	recorder := httptest.NewRecorder()
	service.handleDirectoryDirectMessage(recorder, httptest.NewRequest(http.MethodPost, "/admin/api/directory/direct-message", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("answered %d: %s", recorder.Code, recorder.Body.String())
	}
	return sent
}

func TestAMemberNamingNobodyWritesToTheAgentUnderTheirOwnKey(t *testing.T) {
	sent := sentThroughChatd(t, map[string]string{"senderEmail": "sample@example.com", "message": "안녕하세요"})

	if sent["userSecretHex"] != buzzidentity.Secret("test-seed", "sample@example.com") {
		t.Fatalf("the message was not signed as the member: %+v", sent)
	}
	if _, namesACounterpart := sent["counterpartPubkeyHex"]; namesACounterpart {
		t.Fatalf("chatd addresses the agent when nobody is named, and was handed %+v", sent)
	}
}

func TestAMemberNamingAColleagueWritesToThatColleague(t *testing.T) {
	sent := sentThroughChatd(t, map[string]string{"senderEmail": "sample@example.com", "recipientEmail": "sample@example.com", "message": "안녕하세요"})

	expectedPubkey, _ := nostr.GetPublicKey(buzzidentity.Secret("test-seed", "sample@example.com"))
	if sent["counterpartPubkeyHex"] != expectedPubkey {
		t.Fatalf("the colleague was not addressed: %+v", sent)
	}
}
