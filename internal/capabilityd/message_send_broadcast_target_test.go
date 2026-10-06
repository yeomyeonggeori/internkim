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

func broadcastTargetInput(personHints ...string) json.RawMessage {
	document, _ := json.Marshal(map[string]any{"targetType": "directMessage", "personHints": personHints, "message": "x"})
	return document
}

func broadcastTargetService(t *testing.T) Service {
	t.Helper()
	return messageSendTargetService(t, append(messageSendTargetPeople(), platformDMAmbiguousTestPeople()...))
}

func TestABroadcastOfUniqueNamesResolvesToTheExactIdentityOfEveryRecipient(t *testing.T) {
	service := broadcastTargetService(t)

	target := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", broadcastTargetInput("박예시", "이샘플")))

	if target.InputField != "personHints" || !reflect.DeepEqual(target.IDs, []string{"person-yesi", "person-sample"}) {
		t.Fatalf("the hold narrows personHints to every resolved identity, got %+v", target)
	}
	if !strings.Contains(target.Title, "yesi@example.com") || !strings.Contains(target.Title, "sample@example.com") {
		t.Fatalf("the question names every recipient by address, got %q", target.Title)
	}
}

func TestABroadcastTakesExactIdentifiersAndAddressesAsGiven(t *testing.T) {
	service := broadcastTargetService(t)

	target := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", broadcastTargetInput("person-two", "sample@example.com")))

	if !reflect.DeepEqual(target.IDs, []string{"person-two", "person-sample"}) {
		t.Fatalf("an identifier resolves to itself and an address to its member, got %+v", target)
	}
}

func TestABroadcastNamingOnePersonTwiceNamesThemOnce(t *testing.T) {
	service := broadcastTargetService(t)

	target := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", broadcastTargetInput("박예시", "yesi@example.com")))

	if !reflect.DeepEqual(target.IDs, []string{"person-yesi"}) {
		t.Fatalf("one person receives one message, got %+v", target)
	}
}

func TestTheResolvedBroadcastIdentitiesResolveToThemselves(t *testing.T) {
	service := broadcastTargetService(t)
	byName := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", broadcastTargetInput("박예시", "이샘플")))

	byIdentity := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", broadcastTargetInput(byName.IDs...)))

	if !reflect.DeepEqual(byIdentity, byName) {
		t.Fatalf("narrowing the hold to the identities is only safe while they resolve to themselves, got %+v then %+v", byName, byIdentity)
	}
}

func TestAnAmbiguousBroadcastRecipientAsksWhichByAddressInsteadOfRefusingTheBroadcast(t *testing.T) {
	service := broadcastTargetService(t)

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", broadcastTargetInput("박예시", "lee"))

	assertPlatformDMStructuredFailure(t, response, "error", "interaction_required", "target_resolution", true, true)
	for _, address := range []string{"one@example.com", "two@example.com"} {
		if !strings.Contains(response.Content, address) {
			t.Fatalf("the question names each candidate by address, got %q", response.Content)
		}
	}
	if strings.Contains(response.Content, "yesi@example.com") {
		t.Fatalf("the recipient that resolved is not asked about, got %q", response.Content)
	}
}

func TestAnUnmatchedBroadcastRecipientOffersTheNearestAndNoneOfThese(t *testing.T) {
	service := broadcastTargetService(t)

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", broadcastTargetInput("이샘플", "박예신"))

	assertPlatformDMStructuredFailure(t, response, "error", "interaction_required", "target_resolution", true, true)
	if !strings.Contains(response.Content, "yesi@example.com") || !strings.Contains(response.Content, "none of these") {
		t.Fatalf("a near miss is a question with an exit, got %q", response.Content)
	}
}

func TestABroadcastIdentifierOneCharacterOffIsNeverApproximated(t *testing.T) {
	service := broadcastTargetService(t)

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", broadcastTargetInput("이샘플", "person-yesj"))

	if len(decodeResolvedApprovalTarget(t, response).IDs) != 0 {
		t.Fatalf("an invented identifier resolves to nothing, got %s", response.Result)
	}
}

func TestExecutionSendsABroadcastToExactlyTheResolvedIdentities(t *testing.T) {
	askedKeysFor := []string{}
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch {
		case isDirectoryPeopleRequest(request):
			_ = json.NewEncoder(responseWriter).Encode(map[string]any{"people": append(messageSendTargetPeople(), platformDMAmbiguousTestPeople()...)})
		case request.URL.Path == "/admin/api/directory/buzz-key":
			var asked struct {
				Email string `json:"email"`
			}
			_ = json.NewDecoder(request.Body).Decode(&asked)
			askedKeysFor = append(askedKeysFor, asked.Email)
			_ = json.NewEncoder(responseWriter).Encode(map[string]string{"pubkeyHex": strings.Repeat("2", 64)})
		default:
			postCount++
			_ = json.NewEncoder(responseWriter).Encode(map[string]string{"channelID": "dm-1", "messageID": "post-1"})
		}
	}))
	t.Cleanup(server.Close)
	service := Service{Configuration: Configuration{AdmindBaseURL: server.URL, ChatdEndpoint: server.URL, ChatdPlatform: "buzz"}}
	approvedInput := json.RawMessage(`{"targetType":"directMessage","personHints":["person-one","person-two"],"message":"안녕"}`)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    approvedInput,
		Context:  capabilities.ToolInvokeContext{IsApprovalContinuation: true},
	})

	if errorValue != nil || response.Status != "sent" {
		t.Fatalf("expected the narrowed broadcast to send, got %+v %v", response, errorValue)
	}
	if !reflect.DeepEqual(askedKeysFor, []string{"one@example.com", "two@example.com"}) || postCount != 2 {
		t.Fatalf("the broadcast went to someone other than the approved people: %v, %d posts", askedKeysFor, postCount)
	}
}
