package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type blueclawLedgerAnswering struct {
	mutex       sync.Mutex
	status      int
	redemptions []map[string]any
}

func (ledger *blueclawLedgerAnswering) serve(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/admin/api/held-call/redeem" {
			http.NotFound(responseWriter, request)
			return
		}
		var redemption map[string]any
		_ = json.NewDecoder(request.Body).Decode(&redemption)
		ledger.mutex.Lock()
		ledger.redemptions = append(ledger.redemptions, redemption)
		ledger.mutex.Unlock()
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(ledger.status)
		if ledger.status != http.StatusOK {
			_, _ = responseWriter.Write([]byte(`{"error":"this run holds no call by that id"}`))
			return
		}
		_, _ = responseWriter.Write([]byte(`{"holdID":"held-1"}`))
	}))
	t.Cleanup(server.Close)
	return server.URL
}

type chatdCountingPosts struct {
	mutex sync.Mutex
	posts int
}

func (chatd *chatdCountingPosts) serve(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		chatd.mutex.Lock()
		chatd.posts++
		chatd.mutex.Unlock()
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"messageID":"post-1","channelID":"channel-1"}`))
	}))
	t.Cleanup(server.Close)
	return server.URL
}

const heldChannelPost = `{
	"toolName": "message_send",
	"input": {"targetType":"channel","channelID":"channel-1","message":"내일 10시 회의입니다"},
	"context": {"requesterPersonID":"person-1","holdID":"held-1","taskRunID":"run-1","conversationID":"conversation-1"}
}`

func TestAHoldIDTheLedgerDoesNotRedeemCarriesNothingOut(t *testing.T) {
	ledger := &blueclawLedgerAnswering{status: http.StatusConflict}
	chatd := &chatdCountingPosts{}
	service := Service{Configuration: Configuration{BlueclawBaseURL: ledger.serve(t), ChatdEndpoint: chatd.serve(t), ChatdPlatform: "buzz"}}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "message_send", strings.NewReader(heldChannelPost))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "approval_required" || !strings.Contains(response.Message, "this run holds no call by that id") {
		t.Fatalf("a hold the ledger refused was carried out: %+v", response)
	}
	if chatd.posts != 0 {
		t.Fatalf("chatd was asked to post %d times on a refused hold", chatd.posts)
	}
	if len(ledger.redemptions) != 1 {
		t.Fatalf("the ledger was asked %d times, want 1", len(ledger.redemptions))
	}
}

func TestAHeldCallIsRedeemedAsTheCallItIs(t *testing.T) {
	ledger := &blueclawLedgerAnswering{status: http.StatusOK}
	service := Service{Configuration: Configuration{BlueclawBaseURL: ledger.serve(t)}}
	request, errorValue := decodeToolInvokeRequest("message_send", strings.NewReader(heldChannelPost))
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	_, isDenied := service.capabilityToolApprovalDeniedResponse(context.Background(), request, descriptorForCapabilityToolTest(t, "message_send"))

	if isDenied {
		t.Fatal("a hold the ledger redeemed was refused")
	}
	if len(ledger.redemptions) != 1 {
		t.Fatalf("the ledger was asked %d times, want 1", len(ledger.redemptions))
	}
	redemption := ledger.redemptions[0]
	if redemption["taskRunID"] != "run-1" || redemption["holdID"] != "held-1" || redemption["requesterPersonID"] != "person-1" || redemption["toolName"] != "message_send" {
		t.Fatalf("the ledger was asked about a different call: %+v", redemption)
	}
	if input, _ := json.Marshal(redemption["toolInput"]); string(input) != `{"channelID":"channel-1","message":"내일 10시 회의입니다","targetType":"channel"}` {
		t.Fatalf("the ledger was not shown the input the call carries: %s", input)
	}
}

func TestAHeldCallIsRefusedWhenTheLedgerCannotBeReached(t *testing.T) {
	service := Service{Configuration: Configuration{BlueclawBaseURL: "http://127.0.0.1:1"}}
	request, errorValue := decodeToolInvokeRequest("message_send", strings.NewReader(heldChannelPost))
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	response, isDenied := service.capabilityToolApprovalDeniedResponse(context.Background(), request, descriptorForCapabilityToolTest(t, "message_send"))

	if !isDenied || response.ErrorCode != "approval_required" {
		t.Fatalf("an approval nobody could confirm was taken as given: %+v", response)
	}
}
