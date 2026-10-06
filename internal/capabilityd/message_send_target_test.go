package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func messageSendTargetInput(personHint string) json.RawMessage {
	document, _ := json.Marshal(map[string]string{"targetType": "directMessage", "personHint": personHint, "message": "안녕하세요"})
	return document
}

func messageSendTargetService(t *testing.T, people []directoryPerson) Service {
	t.Helper()
	return platformDMChatdTestService(t, people, &chatdDirectMessageRecorder{})
}

func messageSendTargetPeople() []directoryPerson {
	return []directoryPerson{
		{MemberID: "person-yesi", Email: "yesi@example.com", Name: "박예시"},
		{MemberID: "person-sample", Email: "sample@example.com", Name: "이샘플"},
	}
}

func TestAUniqueRecipientNameResolvesToTheExactPerson(t *testing.T) {
	service := messageSendTargetService(t, messageSendTargetPeople())

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", messageSendTargetInput("박예시"))

	target := decodeResolvedApprovalTarget(t, response)
	if target.InputField != "personHint" || target.ID != "person-yesi" || !strings.Contains(target.Title, "yesi@example.com") {
		t.Fatalf("the approval names exactly who receives it, got %+v", target)
	}
}

func TestTheResolvedRecipientIdentityResolvesToItself(t *testing.T) {
	service := messageSendTargetService(t, messageSendTargetPeople())
	byName := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", messageSendTargetInput("박예시")))

	byIdentity := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", messageSendTargetInput(byName.ID)))

	if !reflect.DeepEqual(byIdentity, byName) {
		t.Fatalf("narrowing the hold to the identity is only safe while it resolves to itself, got %+v then %+v", byName, byIdentity)
	}
}

func TestAnAmbiguousRecipientNameAsksWhichByAddress(t *testing.T) {
	service := messageSendTargetService(t, platformDMAmbiguousTestPeople())

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", messageSendTargetInput("lee"))

	assertPlatformDMStructuredFailure(t, response, "error", "interaction_required", "target_resolution", true, true)
	for _, address := range []string{"one@example.com", "two@example.com"} {
		if !strings.Contains(response.Content, address) {
			t.Fatalf("the question names each candidate by address, got %q", response.Content)
		}
	}
}

func TestANameNothingMatchesOffersTheNearestAndNoneOfThese(t *testing.T) {
	service := messageSendTargetService(t, messageSendTargetPeople())

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", messageSendTargetInput("박예신"))

	assertPlatformDMStructuredFailure(t, response, "error", "interaction_required", "target_resolution", true, true)
	if !strings.Contains(response.Content, "yesi@example.com") || !strings.Contains(response.Content, "none of these") {
		t.Fatalf("a near miss is a question with an exit, got %q", response.Content)
	}
}

func TestAnIdentifierOneCharacterOffIsNeverApproximated(t *testing.T) {
	service := messageSendTargetService(t, messageSendTargetPeople())

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", messageSendTargetInput("person-yesj"))

	if decodeResolvedApprovalTarget(t, response).ID != "" {
		t.Fatalf("an invented identifier resolves to nothing, got %s", response.Result)
	}
}

func TestAReplyInTheCurrentThreadNeedsNoRecipient(t *testing.T) {
	service := messageSendTargetService(t, messageSendTargetPeople())

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", json.RawMessage(`{"targetType":"currentThread","message":"네"}`))

	if response.Status != "no_target" {
		t.Fatalf("expected no target, got %+v", response)
	}
}

func TestExecutionSendsToTheIdentityTheHoldResolved(t *testing.T) {
	people := platformDMAmbiguousTestPeople()
	askedKeysFor := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch {
		case isDirectoryPeopleRequest(request):
			_ = json.NewEncoder(responseWriter).Encode(map[string]any{"people": people})
		case request.URL.Path == "/admin/api/directory/buzz-key":
			var asked struct {
				Email string `json:"email"`
			}
			_ = json.NewDecoder(request.Body).Decode(&asked)
			askedKeysFor = append(askedKeysFor, asked.Email)
			_ = json.NewEncoder(responseWriter).Encode(map[string]string{"pubkeyHex": strings.Repeat("2", 64)})
		default:
			_ = json.NewEncoder(responseWriter).Encode(map[string]string{"channelID": "dm-1", "messageID": "post-1"})
		}
	}))
	t.Cleanup(server.Close)
	service := Service{Configuration: Configuration{AdmindBaseURL: server.URL, ChatdEndpoint: server.URL, ChatdPlatform: "buzz"}}
	approvedInput := json.RawMessage(`{"targetType":"directMessage","personHint":"person-two","message":"안녕"}`)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    approvedInput,
		Context:  capabilities.ToolInvokeContext{IsApprovalContinuation: true},
	})

	if errorValue != nil || response.Status != "sent" {
		t.Fatalf("expected the narrowed call to send, got %+v %v", response, errorValue)
	}
	if len(askedKeysFor) != 1 || askedKeysFor[0] != "two@example.com" {
		t.Fatalf("the message went to someone other than the approved person: %v", askedKeysFor)
	}
}
