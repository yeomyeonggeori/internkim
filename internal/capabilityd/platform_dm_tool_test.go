package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestPlatformDMSendImmediateRunRequiresApprovalContext(t *testing.T) {
	service := Service{}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "message_send", strings.NewReader(`{"input":{"targetType":"directMessage","personHint":"샘플","message":"테스트"},"context":{"requesterPersonID":"person-other","requesterPlatformUserID":"user-other"}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCapabilityApprovalRequired(t, response, "message_send")
}

func TestPlatformDMSendImmediateSelfRequiresDescriptorApproval(t *testing.T) {
	service := Service{}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "message_send", strings.NewReader(`{"input":{"targetType":"directMessage","personHint":"샘플","message":"본인 확인"},"context":{"requesterPersonID":"person-gamyeong","requesterPlatformUserID":"user-gamyeong"}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCapabilityApprovalRequired(t, response, "message_send")
}

func TestResolvePlatformDMRecipientUsesBlueclawResolvedRecipient(t *testing.T) {
	var requestBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if isDirectoryPeopleRequest(request) {
			responseWriter.Header().Set("Content-Type", "application/json")
			_, _ = responseWriter.Write([]byte(directoryPeopleTestDocument))
			return
		}
		if request.Method != http.MethodPost || request.URL.Path != "/admin/api/identity/resolve-recipient" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if errorValue := json.NewDecoder(request.Body).Decode(&requestBody); errorValue != nil {
			t.Fatal(errorValue)
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(platformDMResolvedSampleResponse()))
	}))
	defer server.Close()

	service := Service{Configuration: Configuration{BlueclawBaseURL: server.URL, AdmindBaseURL: server.URL, ChatdPlatform: "buzz"}}
	recipient, failure, hasFailure := service.resolvePlatformDMRecipient(context.Background(), "샘플", "ko")
	if hasFailure {
		t.Fatalf("expected resolved recipient, failure=%+v", failure)
	}
	// The company settles who was meant, so the messenger this device is
	// configured for is asked about an address rather than a half-written name.
	if requestBody["platform"] != "buzz" || requestBody["hint"] != "sample@example.com" {
		t.Fatalf("unexpected resolve request body: %+v", requestBody)
	}
	if recipient.PersonID != "person-gamyeong" || recipient.ExternalUserID != "user-gamyeong" || recipient.Username != "gamyeong" || recipient.Mention != "@gamyeong" {
		t.Fatalf("unexpected resolved recipient: %+v", recipient)
	}
	if strings.Join(recipient.Emails, ",") != "gamyeong@example.com" {
		t.Fatalf("unexpected recipient emails: %+v", recipient.Emails)
	}
}

func TestResolvePlatformDMRecipientReturnsAmbiguousCandidates(t *testing.T) {
	service := platformDMResolverTestService(t, `{"status":"ambiguous","candidates":[{"personID":"person-one","displayName":"Lee One","emails":["one@example.com"],"externalUserID":"user-one"},{"personID":"person-two","displayName":"Lee Two","emails":["two@example.com"],"externalUserID":"user-two"}]}`)

	_, failure, hasFailure := service.resolvePlatformDMRecipient(context.Background(), "lee", "ko")
	if !hasFailure {
		t.Fatal("expected ambiguous failure")
	}
	if failure.ErrorCode != "recipient_ambiguous" || len(failure.Candidates) != 2 {
		t.Fatalf("unexpected ambiguous failure: %+v", failure)
	}
	// Candidates are people, not accounts: which one was meant is settled before
	// any platform is asked which account is theirs.
	if failure.Candidates[0].Emails[0] != "one@example.com" || failure.Candidates[1].Emails[0] != "two@example.com" {
		t.Fatalf("expected the company's candidates: %+v", failure.Candidates)
	}
}

func TestResolvePlatformDMRecipientReturnsNotFound(t *testing.T) {
	service := platformDMResolverTestService(t, `{"status":"not_found","approvedPeople":["이샘플"]}`)

	_, failure, hasFailure := service.resolvePlatformDMRecipient(context.Background(), "없는사람", "ko")
	if !hasFailure {
		t.Fatal("expected not found failure")
	}
	if failure.ErrorCode != "recipient_not_found" || !strings.Contains(failure.Message, `recipient "없는사람" was not found`) {
		t.Fatalf("unexpected not found failure: %+v", failure)
	}
}

func TestResolvePlatformDMRecipientReturnsRetryableFailureWhenBlueclawUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {}))
	server.Close()
	service := Service{Configuration: Configuration{BlueclawBaseURL: server.URL}}

	_, failure, hasFailure := service.resolvePlatformDMRecipient(context.Background(), "샘플", "ko")
	if !hasFailure {
		t.Fatal("expected unavailable failure")
	}
	if failure.ErrorCode != "directory_unavailable" || failure.FailureStage != "recipient_lookup" || !failure.Retryable || !failure.SafeRetry {
		t.Fatalf("unexpected unavailable failure: %+v", failure)
	}
}

func platformDMResolverTestService(t *testing.T, resolutionDocument string) Service {
	t.Helper()
	return platformDMResolverTestServiceForPeople(t, resolutionDocument, platformDMTestDirectoryPeople())
}

// A recipient is named by the company and then looked up on this platform, so a
// test of the second step has to answer the first.
func platformDMResolverTestServiceForPeople(t *testing.T, resolutionDocument string, people []directoryPerson) Service {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if isDirectoryPeopleRequest(request) {
			responseWriter.Header().Set("Content-Type", "application/json")
			_, _ = responseWriter.Write([]byte(directoryPeopleTestDocument))
			return
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodGet && request.URL.Path == "/admin/api/directory/people" {
			_ = json.NewEncoder(responseWriter).Encode(map[string]any{"people": people})
			return
		}
		if request.Method != http.MethodPost || request.URL.Path != "/admin/api/identity/resolve-recipient" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		_, _ = responseWriter.Write([]byte(resolutionDocument))
	}))
	t.Cleanup(server.Close)
	return Service{Configuration: Configuration{BlueclawBaseURL: server.URL, AdmindBaseURL: server.URL}}
}

func platformDMTestDirectoryPeople() []directoryPerson {
	return []directoryPerson{{MemberID: "person-gamyeong", Email: "gamyeong@example.com", Name: "이샘플"}}
}

func platformDMResolvedSampleResponse() string {
	return `{"status":"resolved","recipient":{"personID":"person-gamyeong","displayName":"이샘플","emails":["gamyeong@example.com"],"externalUserID":"user-gamyeong","username":"gamyeong"}}`
}

func TestPlatformMessageBroadcastImmediateRunRequiresApproval(t *testing.T) {
	service := Service{}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "message_send", strings.NewReader(`{"input":{"targetType":"directMessage","personHints":["샘플","정국"],"message":"확인"},"context":{"requesterPersonID":"person-other"}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCapabilityApprovalRequired(t, response, "message_send")
}

func assertPlatformDMStructuredFailure(t *testing.T, response capabilities.ToolInvokeResponse, expectedStatus string, expectedErrorCode string, expectedFailureStage string, expectedRetryable bool, expectedSafeRetry bool) {
	t.Helper()
	if response.Status != expectedStatus || !response.IsError {
		t.Fatalf("expected %s failure response, got %+v", expectedStatus, response)
	}
	expectedOutcome := capabilities.ToolOutcomeFailed
	if expectedStatus == "denied" {
		expectedOutcome = capabilities.ToolOutcomeDenied
	}
	if response.Outcome != expectedOutcome {
		t.Fatalf("expected outcome %q, got %+v", expectedOutcome, response)
	}
	if response.ErrorCode != expectedErrorCode {
		t.Fatalf("expected errorCode %q, got %+v", expectedErrorCode, response)
	}
	if response.FailureStage != expectedFailureStage {
		t.Fatalf("expected failureStage %q, got %+v", expectedFailureStage, response)
	}
	if response.Retryable != expectedRetryable {
		t.Fatalf("expected retryable %v, got %+v", expectedRetryable, response)
	}
	if response.SafeRetry != expectedSafeRetry {
		t.Fatalf("expected safeRetry %v, got %+v", expectedSafeRetry, response)
	}
	var failure platformDMFailure
	if errorValue := json.Unmarshal(response.Result, &failure); errorValue != nil {
		t.Fatal(errorValue)
	}
	if failure.ErrorCode != response.ErrorCode || failure.FailureStage != response.FailureStage {
		t.Fatalf("result failure does not match response: result=%+v response=%+v", failure, response)
	}
}

type chatdDirectMessageRecorder struct {
	messages []string
}

// The company answers who a hint names and what key reaches them; chatd carries
// the message. A direct-message case needs all three, so one stand-in answers
// them and records what chatd was asked to post.
func platformDMChatdTestService(t *testing.T, people []directoryPerson, recorder *chatdDirectMessageRecorder) Service {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		switch {
		case isDirectoryPeopleRequest(request):
			_ = json.NewEncoder(responseWriter).Encode(map[string]any{"people": people})
		case request.URL.Path == "/admin/api/directory/buzz-key":
			_ = json.NewEncoder(responseWriter).Encode(map[string]string{"pubkeyHex": strings.Repeat("2", 64)})
		case request.URL.Path == "/v1/platform/buzz/dm.post":
			var posted struct {
				Message string `json:"message"`
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&posted); errorValue != nil {
				t.Fatal(errorValue)
			}
			recorder.messages = append(recorder.messages, posted.Message)
			_ = json.NewEncoder(responseWriter).Encode(map[string]string{"channelID": "dm-1", "messageID": "post-1"})
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	return Service{Configuration: Configuration{AdmindBaseURL: server.URL, ChatdEndpoint: server.URL, ChatdPlatform: "buzz"}}
}

func platformDMChatdTestPeople() []directoryPerson {
	return []directoryPerson{
		{MemberID: "person-sample", Email: "sample@example.com", Name: "이샘플"},
		{MemberID: "person-jungkook", Email: "jungkook@example.com", Name: "전정국"},
	}
}

func platformDMAmbiguousTestPeople() []directoryPerson {
	return []directoryPerson{
		{MemberID: "person-one", Email: "one@example.com", Name: "Lee One"},
		{MemberID: "person-two", Email: "two@example.com", Name: "Lee Two"},
	}
}

func TestPlatformDMSendApprovedContinuationSendsThroughChatd(t *testing.T) {
	recorder := &chatdDirectMessageRecorder{}
	service := platformDMChatdTestService(t, platformDMChatdTestPeople(), recorder)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    []byte(`{"targetType":"directMessage","personHint":"샘플","message":"승인 후 전송"}`),
		Context: capabilities.ToolInvokeContext{
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "sent" || response.IsError {
		t.Fatalf("expected approved continuation to send, got %+v", response)
	}
	if !reflect.DeepEqual(recorder.messages, []string{"승인 후 전송"}) {
		t.Fatalf("unexpected chatd posts: %+v", recorder.messages)
	}
}

func TestPlatformDMSendAmbiguousRecipientDoesNotSend(t *testing.T) {
	recorder := &chatdDirectMessageRecorder{}
	service := platformDMChatdTestService(t, platformDMAmbiguousTestPeople(), recorder)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    []byte(`{"targetType":"directMessage","personHint":"lee","message":"테스트"}`),
		Context: capabilities.ToolInvokeContext{
			IsScheduledRun: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "error" || !strings.Contains(response.Content, "ambiguous") {
		t.Fatalf("expected ambiguous error, got %+v", response)
	}
	assertPlatformDMStructuredFailure(t, response, "error", "recipient_ambiguous", "recipient_resolve", false, false)
	if len(recorder.messages) != 0 {
		t.Fatalf("an ambiguous recipient was posted to: %+v", recorder.messages)
	}
}

func TestPlatformMessageSendAmbiguousRecipientReturnsCandidatesWithoutSending(t *testing.T) {
	recorder := &chatdDirectMessageRecorder{}
	service := platformDMChatdTestService(t, platformDMAmbiguousTestPeople(), recorder)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    []byte(`{"targetType":"directMessage","personHint":"lee","message":"테스트"}`),
		Context:  capabilities.ToolInvokeContext{IsScheduledRun: true},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "error" || response.ErrorCode != "recipient_ambiguous" {
		t.Fatalf("expected ambiguous send failure, got %+v", response)
	}
	if len(recorder.messages) != 0 {
		t.Fatalf("an ambiguous recipient was posted to: %+v", recorder.messages)
	}
	var failure platformDMFailure
	if errorValue := json.Unmarshal(response.Result, &failure); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(failure.Candidates) != 2 {
		t.Fatalf("expected two candidates, got %+v", failure)
	}
	if failure.Candidates[0].PersonID == "" || failure.Candidates[1].PersonID == "" {
		t.Fatalf("expected the company's candidates, got %+v", failure.Candidates)
	}
}

func TestPlatformMessageBroadcastFansOutWithPerRecipientRollup(t *testing.T) {
	recorder := &chatdDirectMessageRecorder{}
	service := platformDMChatdTestService(t, platformDMChatdTestPeople(), recorder)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    []byte(`{"targetType":"directMessage","personHints":["샘플","정국","없는사람"],"message":"완료 확인 부탁"}`),
		Context:  capabilities.ToolInvokeContext{IsApprovalContinuation: true},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "sent" || response.IsError {
		t.Fatalf("expected partial success response, got %+v", response)
	}
	if len(recorder.messages) != 2 {
		t.Fatalf("expected two posts for two resolved recipients, got %+v", recorder.messages)
	}
	var result platformMessageSendResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !reflect.DeepEqual(result.MessageIDs, []string{"post-1"}) || result.DeliveryStatus != "sent" || len(result.Failures) != 1 {
		t.Fatalf("expected canonical successful message IDs and one failure, got %+v", result)
	}
	if result.Failures[0].PersonHint != "없는사람" || result.Failures[0].ErrorCode != "recipient_not_found" {
		t.Fatalf("expected not-found recipient diagnostic, got %+v", result.Failures)
	}
}

func TestPlatformMessageSendTellsTwoPeopleWithOneNameApartByAddress(t *testing.T) {
	recorder := &chatdDirectMessageRecorder{}
	service := platformDMChatdTestService(t, []directoryPerson{
		{MemberID: "person-work", Email: "sample@example.com", Name: "이샘플(work)"},
		{MemberID: "person-personal", Email: "sample2468@example.com", Name: "샘플 이"},
	}, recorder)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    []byte(`{"targetType":"directMessage","personHint":"이샘플","message":"안녕"}`),
		Context:  capabilities.ToolInvokeContext{IsScheduledRun: true},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "error" || response.ErrorCode != "recipient_ambiguous" {
		t.Fatalf("two people with one name are a question, got %+v", response)
	}
	for _, address := range []string{"sample@example.com", "sample2468@example.com"} {
		if !strings.Contains(response.Content, address) {
			t.Fatalf("the question tells the two apart by address, got %q", response.Content)
		}
	}
	if len(recorder.messages) != 0 {
		t.Fatalf("nothing goes out while the question stands: %+v", recorder.messages)
	}
}
