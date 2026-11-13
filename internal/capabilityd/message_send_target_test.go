package capabilityd

import (
	"bytes"
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
	if target.InputField != "personHint" || target.ID != "person-yesi" || target.Title != "박예시" {
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
		Context:  capabilities.ToolInvokeContext{HoldID: "held-test"},
	})

	if errorValue != nil || response.Status != "sent" {
		t.Fatalf("expected the narrowed call to send, got %+v %v", response, errorValue)
	}
	if len(askedKeysFor) != 1 || askedKeysFor[0] != "two@example.com" {
		t.Fatalf("the message went to someone other than the approved person: %v", askedKeysFor)
	}
}

func sameNamedPeople() []directoryPerson {
	return []directoryPerson{
		{MemberID: "person-sample-one", Email: "sample1@example.com", Name: "샘플 이"},
		{MemberID: "person-sample-two", Email: "sample2@example.com", Name: "샘플 이"},
		{MemberID: "person-yesi", Email: "yesi@example.com", Name: "예시 박"},
	}
}

func resolveMessageSendTargetIn(t *testing.T, service Service, input json.RawMessage, language string) capabilities.ToolInvokeResponse {
	t.Helper()
	request := approvalTargetInvokeRequest("message_send", input)
	request.Context.ResponseLanguage = language
	requestDocument, _ := json.Marshal(request)
	response, errorValue := service.resolveCapabilityToolTarget(context.Background(), "message_send", bytes.NewReader(requestDocument))
	if errorValue != nil {
		t.Fatalf("expected the target resolution route to answer: %v", errorValue)
	}
	return response
}

func TestEveryPrintedRecipientLineNamesThatPersonOrAsksNeverSomeoneElse(t *testing.T) {
	service := messageSendTargetService(t, sameNamedPeople())
	for _, language := range []string{"ko", "en"} {
		for _, person := range sameNamedPeople() {
			target := decodeResolvedApprovalTarget(t, resolveMessageSendTargetIn(t, service, messageSendTargetInput(person.Email), language))
			for _, printedLine := range []string{target.Title, target.Address} {
				if printedLine == "" {
					continue
				}
				response := resolveMessageSendTargetIn(t, service, messageSendTargetInput(printedLine), language)
				if response.IsError {
					assertPlatformDMStructuredFailure(t, response, "error", "interaction_required", "target_resolution", true, true)
					continue
				}
				if resolved := decodeResolvedApprovalTarget(t, response); resolved.ID != person.MemberID {
					t.Fatalf("the line %q printed for %s reaches %s instead", printedLine, person.MemberID, resolved.ID)
				}
			}
		}
	}
}

func TestOnlyARecipientWhoseNameIsSharedIsPrintedWithAnAddress(t *testing.T) {
	service := messageSendTargetService(t, sameNamedPeople())

	shared := decodeResolvedApprovalTarget(t, resolveMessageSendTargetIn(t, service, messageSendTargetInput("person-sample-two"), "ko"))
	unique := decodeResolvedApprovalTarget(t, resolveMessageSendTargetIn(t, service, messageSendTargetInput("person-yesi"), "ko"))

	if shared.Title != "이샘플" || shared.Address != "sample2@example.com" {
		t.Fatalf("a name two people answer to is printed with the address that tells them apart, on its own line, got %+v", shared)
	}
	if unique.Title != "박예시" || unique.Address != "" {
		t.Fatalf("a name only one person answers to needs no address, got %+v", unique)
	}
}

func TestSeveralRecipientsCarryTheAddressesOfThoseWhoseNamesAreShared(t *testing.T) {
	service := messageSendTargetService(t, sameNamedPeople())
	input := json.RawMessage(`{"targetType":"directMessage","personHints":["sample1@example.com","박예시"],"message":"안녕하세요"}`)

	target := decodeResolvedApprovalTarget(t, resolveMessageSendTargetIn(t, service, input, "ko"))

	if target.Title != "이샘플; 박예시" || target.Address != "sample1@example.com" {
		t.Fatalf("only the shared name needs its address, got %+v", target)
	}
}
