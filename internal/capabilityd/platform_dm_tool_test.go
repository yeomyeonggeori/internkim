package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestPlatformDMSendScheduledRunSendsMattermostDM(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	requestPaths := []string{}
	var directChannelBody []string
	var postBody map[string]string
	service := Service{
		Configuration: Configuration{
			BlueclawBaseURL:     "http://blueclaw.local",
			MattermostBaseURL:   "http://mattermost.local",
			MattermostTokenPath: tokenPath,
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestPaths = append(requestPaths, request.URL.String())
			switch request.URL.String() {
			case "http://blueclaw.local/admin/api/identity/resolve-recipient":
				return platformDMTestJSONResponse(platformDMResolvedDonghaResponse()), nil
			case "http://mattermost.local/api/v4/users/me":
				return platformDMTestJSONResponse(`{"id":"bot-user","username":"internkim","is_bot":true}`), nil
			case "http://mattermost.local/api/v4/channels/direct":
				if errorValue := json.NewDecoder(request.Body).Decode(&directChannelBody); errorValue != nil {
					t.Fatal(errorValue)
				}
				return platformDMTestJSONResponse(`{"id":"dm-channel-1"}`), nil
			case "http://mattermost.local/api/v4/posts":
				if errorValue := json.NewDecoder(request.Body).Decode(&postBody); errorValue != nil {
					t.Fatal(errorValue)
				}
				return platformDMTestJSONResponse(`{"id":"post-1"}`), nil
			default:
				t.Fatalf("unexpected request %s", request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    []byte(`{"targetType":"directMessage","personHint":"동하","message":"테스트"}`),
		Context: capabilities.ToolInvokeContext{
			IsScheduledRun: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "sent" || response.Outcome != capabilities.ToolOutcomeSucceeded || response.IsError || len(response.Effects) != 1 || response.Effects[0].ID != "post-1" {
		t.Fatalf("expected sent response, got %+v", response)
	}
	var result platformMessageSendResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !reflect.DeepEqual(result.MessageIDs, []string{"post-1"}) || result.DeliveryStatus != "sent" {
		t.Fatalf("unexpected canonical send result: %+v", result)
	}
	if strings.Join(directChannelBody, ",") != "user-dongha,bot-user" {
		t.Fatalf("unexpected direct channel body: %+v", directChannelBody)
	}
	if postBody["channel_id"] != "dm-channel-1" || postBody["message"] != "테스트" {
		t.Fatalf("unexpected post body: %+v", postBody)
	}
	if len(requestPaths) != 4 {
		t.Fatalf("expected four requests, got %+v", requestPaths)
	}
}

func TestPlatformDMSendImmediateRunRequiresApprovalContext(t *testing.T) {
	service := Service{}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "message_send", strings.NewReader(`{"input":{"targetType":"directMessage","personHint":"동하","message":"테스트"},"context":{"requesterPersonID":"person-other","requesterPlatformUserID":"user-other"}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCapabilityApprovalRequired(t, response, "message_send")
}

func TestPlatformDMSendImmediateSelfRequiresDescriptorApproval(t *testing.T) {
	service := Service{}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "message_send", strings.NewReader(`{"input":{"targetType":"directMessage","personHint":"동하","message":"본인 확인"},"context":{"requesterPersonID":"person-dongha","requesterPlatformUserID":"user-dongha"}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCapabilityApprovalRequired(t, response, "message_send")
}

func TestPlatformDMSendApprovedContinuationSendsMattermostDM(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, platformDMResolvedDonghaResponse())

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    []byte(`{"targetType":"directMessage","personHint":"@dongha","message":"승인 후 전송"}`),
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
}

func TestPlatformDMSendMatchesMattermostNickname(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, `{"status":"resolved","recipient":{"personID":"person-dongha","displayName":"이동하","emails":["dongha@example.com"],"externalUserID":"user-dongha","username":"member-42"}}`)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    []byte(`{"targetType":"directMessage","personHint":"동하","message":"테스트"}`),
		Context: capabilities.ToolInvokeContext{
			IsScheduledRun: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "sent" || response.IsError {
		t.Fatalf("expected Mattermost nickname match, got %+v", response)
	}
}

func TestResolvePlatformDMRecipientUsesBlueclawResolvedRecipient(t *testing.T) {
	var requestBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/admin/api/identity/resolve-recipient" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if errorValue := json.NewDecoder(request.Body).Decode(&requestBody); errorValue != nil {
			t.Fatal(errorValue)
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(platformDMResolvedDonghaResponse()))
	}))
	defer server.Close()

	service := Service{Configuration: Configuration{BlueclawBaseURL: server.URL}}
	recipient, failure, hasFailure := service.resolvePlatformDMRecipient(context.Background(), "동하")
	if hasFailure {
		t.Fatalf("expected resolved recipient, failure=%+v", failure)
	}
	if requestBody["platform"] != "mattermost" || requestBody["hint"] != "동하" {
		t.Fatalf("unexpected resolve request body: %+v", requestBody)
	}
	if recipient.PersonID != "person-dongha" || recipient.MattermostUserID != "user-dongha" || recipient.MattermostUsername != "dongha" || recipient.Mention != "@dongha" {
		t.Fatalf("unexpected resolved recipient: %+v", recipient)
	}
	if strings.Join(recipient.Emails, ",") != "dongha@example.com" {
		t.Fatalf("unexpected recipient emails: %+v", recipient.Emails)
	}
}

func TestResolvePlatformDMRecipientReturnsAmbiguousCandidates(t *testing.T) {
	service := platformDMResolverTestService(t, `{"status":"ambiguous","candidates":[{"personID":"person-one","displayName":"Lee One","emails":["one@example.com"],"externalUserID":"user-one"},{"personID":"person-two","displayName":"Lee Two","emails":["two@example.com"],"externalUserID":"user-two"}]}`)

	_, failure, hasFailure := service.resolvePlatformDMRecipient(context.Background(), "lee")
	if !hasFailure {
		t.Fatal("expected ambiguous failure")
	}
	if failure.ErrorCode != "recipient_ambiguous" || len(failure.Candidates) != 2 {
		t.Fatalf("unexpected ambiguous failure: %+v", failure)
	}
	if failure.Candidates[0].MattermostUserID != "user-one" || failure.Candidates[1].MattermostUserID != "user-two" {
		t.Fatalf("expected external user ids in candidates: %+v", failure.Candidates)
	}
}

func TestResolvePlatformDMRecipientReturnsNotFound(t *testing.T) {
	service := platformDMResolverTestService(t, `{"status":"not_found","approvedPeople":["이동하"]}`)

	_, failure, hasFailure := service.resolvePlatformDMRecipient(context.Background(), "없는사람")
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

	_, failure, hasFailure := service.resolvePlatformDMRecipient(context.Background(), "동하")
	if !hasFailure {
		t.Fatal("expected unavailable failure")
	}
	if failure.ErrorCode != "mattermost_unavailable" || failure.FailureStage != "mattermost_lookup" || !failure.Retryable || !failure.SafeRetry {
		t.Fatalf("unexpected unavailable failure: %+v", failure)
	}
}

func TestPlatformDMSendAmbiguousRecipientDoesNotSend(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, `{"status":"ambiguous","candidates":[{"personID":"person-one","displayName":"Lee One","emails":["one@example.com"],"externalUserID":"user-one"},{"personID":"person-two","displayName":"Lee Two","emails":["two@example.com"],"externalUserID":"user-two"}]}`)

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
}

func TestPlatformDMSendMissingMattermostTokenDoesNotSend(t *testing.T) {
	service := platformDMTestService(t, "/missing/token", platformDMResolvedDonghaResponse())

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    []byte(`{"targetType":"directMessage","personHint":"동하","message":"테스트"}`),
		Context: capabilities.ToolInvokeContext{
			IsScheduledRun: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "error" || !strings.Contains(response.Content, "mattermost bot token is not configured") {
		t.Fatalf("expected token error, got %+v", response)
	}
	assertPlatformDMStructuredFailure(t, response, "error", "mattermost_unavailable", "mattermost_lookup", false, false)
}

func TestPlatformDMSendPostFailureIsNotSafeToRetry(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := Service{
		Configuration: Configuration{
			BlueclawBaseURL:     "http://blueclaw.local",
			MattermostBaseURL:   "http://mattermost.local",
			MattermostTokenPath: tokenPath,
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch request.URL.String() {
			case "http://blueclaw.local/admin/api/identity/resolve-recipient":
				return platformDMTestJSONResponse(platformDMResolvedDonghaResponse()), nil
			case "http://mattermost.local/api/v4/users/me":
				return platformDMTestJSONResponse(`{"id":"bot-user","username":"internkim","is_bot":true}`), nil
			case "http://mattermost.local/api/v4/channels/direct":
				return platformDMTestJSONResponse(`{"id":"dm-channel-1"}`), nil
			case "http://mattermost.local/api/v4/posts":
				return platformDMTestStatusResponse(http.StatusServiceUnavailable, "mattermost unavailable"), nil
			default:
				t.Fatalf("unexpected request %s", request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    []byte(`{"targetType":"directMessage","personHint":"동하","message":"테스트"}`),
		Context: capabilities.ToolInvokeContext{
			IsScheduledRun: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertPlatformDMStructuredFailure(t, response, "error", "send_failed", "message_send", true, false)
}

func TestPlatformDMSendDirectChannelFailureUsesSpecificStage(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := Service{
		Configuration: Configuration{
			BlueclawBaseURL:     "http://blueclaw.local",
			MattermostBaseURL:   "http://mattermost.local",
			MattermostTokenPath: tokenPath,
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch request.URL.String() {
			case "http://blueclaw.local/admin/api/identity/resolve-recipient":
				return platformDMTestJSONResponse(platformDMResolvedDonghaResponse()), nil
			case "http://mattermost.local/api/v4/users/me":
				return platformDMTestJSONResponse(`{"id":"bot-user","username":"internkim","is_bot":true}`), nil
			case "http://mattermost.local/api/v4/channels/direct":
				return platformDMTestStatusResponse(http.StatusServiceUnavailable, "direct channel unavailable"), nil
			default:
				t.Fatalf("unexpected request %s", request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    []byte(`{"targetType":"directMessage","personHint":"동하","message":"테스트"}`),
		Context: capabilities.ToolInvokeContext{
			IsScheduledRun: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertPlatformDMStructuredFailure(t, response, "error", "direct_channel_create_failed", "direct_channel_create", true, true)
}

func TestPlatformMessageSendAmbiguousRecipientReturnsCandidatesWithoutSending(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	requestPaths := []string{}
	service := Service{
		Configuration: Configuration{
			BlueclawBaseURL:     "http://blueclaw.local",
			MattermostBaseURL:   "http://mattermost.local",
			MattermostTokenPath: tokenPath,
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestPaths = append(requestPaths, request.URL.String())
			switch request.URL.String() {
			case "http://blueclaw.local/admin/api/identity/resolve-recipient":
				return platformDMTestJSONResponse(`{"status":"ambiguous","candidates":[{"personID":"person-one","displayName":"Lee One","emails":["one@example.com"],"externalUserID":"user-one"},{"personID":"person-two","displayName":"Lee Two","emails":["two@example.com"],"externalUserID":"user-two"}]}`), nil
			case "http://mattermost.local/api/v4/users/me":
				return platformDMTestJSONResponse(`{"id":"bot-user","username":"internkim","is_bot":true}`), nil
			default:
				t.Fatalf("unexpected request %s", request.URL.String())
				return nil, nil
			}
		})},
	}

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
	if strings.Contains(strings.Join(requestPaths, "\n"), "/api/v4/posts") {
		t.Fatalf("ambiguous send posted a message: %+v", requestPaths)
	}
	if strings.Contains(strings.Join(requestPaths, "\n"), "/api/v4/channels/direct") {
		t.Fatalf("ambiguous send created a direct channel: %+v", requestPaths)
	}
	assertPlatformDMRequestCount(t, requestPaths, "http://blueclaw.local/admin/api/identity/resolve-recipient", 1)
	var failure platformDMFailure
	if errorValue := json.Unmarshal(response.Result, &failure); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(failure.Candidates) != 2 {
		t.Fatalf("expected two candidates, got %+v", failure)
	}
	if failure.Candidates[0].MattermostUserID == "" || failure.Candidates[1].MattermostUserID == "" {
		t.Fatalf("expected Mattermost candidate ids, got %+v", failure.Candidates)
	}
}

func platformDMTestService(t *testing.T, tokenPath string, resolutionDocument string) Service {
	t.Helper()
	return Service{
		Configuration: Configuration{
			BlueclawBaseURL:     "http://blueclaw.local",
			MattermostBaseURL:   "http://mattermost.local",
			MattermostTokenPath: tokenPath,
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch request.URL.String() {
			case "http://blueclaw.local/admin/api/identity/resolve-recipient":
				return platformDMTestJSONResponse(resolutionDocument), nil
			case "http://mattermost.local/api/v4/users/me":
				return platformDMTestJSONResponse(`{"id":"bot-user","username":"internkim","is_bot":true}`), nil
			case "http://mattermost.local/api/v4/channels/direct":
				return platformDMTestJSONResponse(`{"id":"dm-channel-1"}`), nil
			case "http://mattermost.local/api/v4/posts":
				return platformDMTestJSONResponse(`{"id":"post-1"}`), nil
			default:
				t.Fatalf("unexpected request %s", request.URL.String())
				return nil, nil
			}
		})},
	}
}

func platformDMResolverTestService(t *testing.T, resolutionDocument string) Service {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/admin/api/identity/resolve-recipient" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(resolutionDocument))
	}))
	t.Cleanup(server.Close)
	return Service{Configuration: Configuration{BlueclawBaseURL: server.URL}}
}

func platformDMResolvedDonghaResponse() string {
	return `{"status":"resolved","recipient":{"personID":"person-dongha","displayName":"이동하","emails":["dongha@example.com"],"externalUserID":"user-dongha","username":"dongha"}}`
}

func writePlatformDMTestFile(t *testing.T, value string) string {
	t.Helper()
	path := t.TempDir() + "/secret"
	if errorValue := os.WriteFile(path, []byte(value), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}

func TestPlatformMessageBroadcastFansOutWithPerRecipientRollup(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	postCount := 0
	service := Service{
		Configuration: Configuration{
			BlueclawBaseURL:     "http://blueclaw.local",
			MattermostBaseURL:   "http://mattermost.local",
			MattermostTokenPath: tokenPath,
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch request.URL.String() {
			case "http://blueclaw.local/admin/api/identity/resolve-recipient":
				var body struct {
					Hint string `json:"hint"`
				}
				if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil {
					t.Fatal(errorValue)
				}
				switch body.Hint {
				case "동하":
					return platformDMTestJSONResponse(platformDMResolvedDonghaResponse()), nil
				case "정국":
					return platformDMTestJSONResponse(`{"status":"resolved","recipient":{"personID":"person-jungkook","displayName":"전정국","emails":["jk@example.com"],"externalUserID":"user-jungkook","username":"jk"}}`), nil
				default:
					return platformDMTestJSONResponse(`{"status":"not_found","approvedPeople":["이동하"]}`), nil
				}
			case "http://mattermost.local/api/v4/users/me":
				return platformDMTestJSONResponse(`{"id":"bot-user","username":"internkim","is_bot":true}`), nil
			case "http://mattermost.local/api/v4/channels/direct":
				return platformDMTestJSONResponse(`{"id":"dm-channel"}`), nil
			case "http://mattermost.local/api/v4/posts":
				postCount++
				return platformDMTestJSONResponse(`{"id":"post-1"}`), nil
			default:
				t.Fatalf("unexpected request %s", request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    []byte(`{"targetType":"directMessage","personHints":["동하","정국","없는사람"],"message":"완료 확인 부탁"}`),
		Context:  capabilities.ToolInvokeContext{IsApprovalContinuation: true},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "sent" || response.IsError {
		t.Fatalf("expected partial success response, got %+v", response)
	}
	if postCount != 2 {
		t.Fatalf("expected two posts for two resolved recipients, got %d", postCount)
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

func TestPlatformMessageBroadcastImmediateRunRequiresApproval(t *testing.T) {
	service := Service{}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "message_send", strings.NewReader(`{"input":{"targetType":"directMessage","personHints":["동하","정국"],"message":"확인"},"context":{"requesterPersonID":"person-other"}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCapabilityApprovalRequired(t, response, "message_send")
}

func platformDMTestJSONResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}

func platformDMTestStatusResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
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

func assertPlatformDMRequestCount(t *testing.T, requestPaths []string, expectedPath string, expectedCount int) {
	t.Helper()
	count := 0
	for _, requestPath := range requestPaths {
		if requestPath == expectedPath {
			count++
		}
	}
	if count != expectedCount {
		t.Fatalf("expected %s to be requested %d time(s), got %d in %+v", expectedPath, expectedCount, count, requestPaths)
	}
}

func TestMattermostPendingPostIDComposition(t *testing.T) {
	if pending := mattermostPendingPostID("bot-1", "key-abc"); pending != "bot-1:key-abc" {
		t.Fatalf("pending post id = %q", pending)
	}
	if pending := mattermostPendingPostID("bot-1", ""); pending != "" {
		t.Fatalf("expected empty pending post id without key, got %q", pending)
	}
	if pending := mattermostPendingPostID("", "key-abc"); pending != "" {
		t.Fatalf("expected empty pending post id without bot, got %q", pending)
	}
}

func TestSendMattermostDirectMessageSetsPendingPostID(t *testing.T) {
	var sentPendingPostID string
	service := Service{HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/api/v4/users/me"):
			return jsonResponse(map[string]any{"id": "bot-1", "username": "internkim", "is_bot": true}), nil
		case strings.HasSuffix(request.URL.Path, "/api/v4/channels/direct"):
			return jsonResponse(map[string]string{"id": "channel-1"}), nil
		case strings.HasSuffix(request.URL.Path, "/api/v4/posts") && request.Method == http.MethodPost:
			var body map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil {
				t.Fatal(errorValue)
			}
			sentPendingPostID = body["pending_post_id"]
			return jsonResponse(map[string]string{"id": "post-1"}), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}, Configuration: Configuration{MattermostBaseURL: "http://mattermost.test", MattermostTokenPath: writePlatformTestFile(t, "bot-token")}}

	dispatchID, _, hasFailure := service.sendMattermostDirectMessageWithDispatch(context.Background(), "user-9", "안녕", "key-xyz")
	if hasFailure {
		t.Fatal("unexpected send failure")
	}
	if dispatchID != "post-1" {
		t.Fatalf("dispatch id = %q", dispatchID)
	}
	if sentPendingPostID != "bot-1:key-xyz" {
		t.Fatalf("pending post id = %q", sentPendingPostID)
	}
}
