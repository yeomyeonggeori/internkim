package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
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
			case "http://blueclaw.local/admin/api/policy":
				return platformDMTestJSONResponse(`{"people":[{"personID":"person-gamyeong","displayName":"이샘플","emails":["gamyeong@example.com"]}]}`), nil
			case "http://mattermost.local/api/v4/users?per_page=200":
				return platformDMTestJSONResponse(`[{"id":"user-gamyeong","email":"gamyeong@example.com","username":"gamyeong","nickname":"샘플"}]`), nil
			case "http://mattermost.local/api/v4/users/me":
				return platformDMTestJSONResponse(`{"id":"bot-user","username":"internkim"}`), nil
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
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"샘플"},"message":"테스트"}`),
		Context: capabilities.ToolInvokeContext{
			IsScheduledRun: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "sent" || response.IsError {
		t.Fatalf("expected sent response, got %+v", response)
	}
	if strings.Join(directChannelBody, ",") != "user-gamyeong,bot-user" {
		t.Fatalf("unexpected direct channel body: %+v", directChannelBody)
	}
	if postBody["channel_id"] != "dm-channel-1" || postBody["message"] != "테스트" {
		t.Fatalf("unexpected post body: %+v", postBody)
	}
	if len(requestPaths) != 5 {
		t.Fatalf("expected five requests, got %+v", requestPaths)
	}
}

func TestPlatformDMSendImmediateRunRequiresApprovalContext(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, `[{"id":"user-gamyeong","email":"gamyeong@example.com","username":"gamyeong"}]`)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"샘플"},"message":"테스트"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterPersonID:       "person-other",
			RequesterPlatformUserID: "user-other",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "denied" || !response.IsError {
		t.Fatalf("expected denied response, got %+v", response)
	}
	assertPlatformDMStructuredFailure(t, response, "denied", "approval_required", "authorization", false, false)
}

func TestPlatformDMSendImmediateSelfDoesNotRequireApproval(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, `[{"id":"user-gamyeong","email":"gamyeong@example.com","username":"gamyeong","nickname":"샘플"}]`)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"샘플"},"message":"본인 확인"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterPersonID:       "person-gamyeong",
			RequesterPlatformUserID: "user-gamyeong",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "sent" || response.IsError {
		t.Fatalf("expected self DM to send without approval, got %+v", response)
	}
}

func TestPlatformDMSendApprovedContinuationSendsMattermostDM(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, `[{"id":"user-gamyeong","email":"gamyeong@example.com","username":"gamyeong"}]`)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"@gamyeong"},"message":"승인 후 전송"}`),
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
	service := platformDMTestService(t, tokenPath, `[{"id":"user-gamyeong","email":"gamyeong@example.com","username":"member-42","nickname":"샘플"}]`)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"샘플"},"message":"테스트"}`),
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

func TestMatchingPlatformDMRecipientsMatchesPartialKoreanGivenName(t *testing.T) {
	matches := matchingPlatformDMRecipients("테스트", []platformDMPolicyPerson{{
		PersonID:    "person-siheom",
		DisplayName: "김테스트",
		Emails:      []string{"siheom@example.com"},
	}}, []platformDMMattermostUser{{
		ID:    "user-siheom",
		Email: "siheom@example.com",
	}})

	if len(matches) != 1 || matches[0].PersonID != "person-siheom" {
		t.Fatalf("expected Siheom match, got %+v", matches)
	}
}

func TestMatchingPlatformDMRecipientsReturnsMultiplePartialKoreanMatches(t *testing.T) {
	matches := matchingPlatformDMRecipients("테스트", []platformDMPolicyPerson{{
		PersonID:    "person-siheom-a",
		DisplayName: "김테스트",
		Emails:      []string{"siheom-a@example.com"},
	}, {
		PersonID:    "person-siheom-b",
		DisplayName: "김테스트",
		Emails:      []string{"siheom-b@example.com"},
	}}, []platformDMMattermostUser{{
		ID:    "user-siheom-a",
		Email: "siheom-a@example.com",
	}, {
		ID:    "user-siheom-b",
		Email: "siheom-b@example.com",
	}})

	if len(matches) != 2 {
		t.Fatalf("expected ambiguous Siheom matches, got %+v", matches)
	}
}

func TestMatchingPlatformDMRecipientsMatchesFullName(t *testing.T) {
	matches := matchingPlatformDMRecipients("김테스트", []platformDMPolicyPerson{{
		PersonID:    "person-siheom",
		DisplayName: "김테스트",
		Emails:      []string{"siheom@example.com"},
	}}, []platformDMMattermostUser{{
		ID:    "user-siheom",
		Email: "siheom@example.com",
	}})

	if len(matches) != 1 || matches[0].MattermostUserID != "user-siheom" {
		t.Fatalf("expected full name match, got %+v", matches)
	}
}

func TestMatchingPlatformDMRecipientsReturnsNoMatch(t *testing.T) {
	matches := matchingPlatformDMRecipients("샘플", []platformDMPolicyPerson{{
		PersonID:    "person-siheom",
		DisplayName: "김테스트",
		Emails:      []string{"siheom@example.com"},
	}}, []platformDMMattermostUser{{
		ID:    "user-siheom",
		Email: "siheom@example.com",
	}})

	if len(matches) != 0 {
		t.Fatalf("expected no match, got %+v", matches)
	}
}

func TestPlatformDMSendAmbiguousRecipientDoesNotSend(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, `[
		{"id":"user-one","email":"one@example.com","username":"lee"},
		{"id":"user-two","email":"two@example.com","username":"lee-two"}
	]`)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"lee"},"message":"테스트"}`),
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
	service := platformDMTestService(t, "/missing/token", `[{"id":"user-gamyeong","email":"gamyeong@example.com","username":"gamyeong"}]`)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"샘플"},"message":"테스트"}`),
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
			case "http://blueclaw.local/admin/api/policy":
				return platformDMTestJSONResponse(`{"people":[{"personID":"person-gamyeong","displayName":"이샘플","emails":["gamyeong@example.com"]}]}`), nil
			case "http://mattermost.local/api/v4/users?per_page=200":
				return platformDMTestJSONResponse(`[{"id":"user-gamyeong","email":"gamyeong@example.com","username":"gamyeong"}]`), nil
			case "http://mattermost.local/api/v4/users/me":
				return platformDMTestJSONResponse(`{"id":"bot-user","username":"internkim"}`), nil
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
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"샘플"},"message":"테스트"}`),
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
			case "http://blueclaw.local/admin/api/policy":
				return platformDMTestJSONResponse(`{"people":[{"personID":"person-gamyeong","displayName":"이샘플","emails":["gamyeong@example.com"]}]}`), nil
			case "http://mattermost.local/api/v4/users?per_page=200":
				return platformDMTestJSONResponse(`[{"id":"user-gamyeong","email":"gamyeong@example.com","username":"gamyeong"}]`), nil
			case "http://mattermost.local/api/v4/users/me":
				return platformDMTestJSONResponse(`{"id":"bot-user","username":"internkim"}`), nil
			case "http://mattermost.local/api/v4/channels/direct":
				return platformDMTestStatusResponse(http.StatusServiceUnavailable, "direct channel unavailable"), nil
			default:
				t.Fatalf("unexpected request %s", request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"샘플"},"message":"테스트"}`),
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
			case "http://blueclaw.local/admin/api/policy":
				return platformDMTestJSONResponse(`{"people":[{"personID":"person-one","displayName":"Lee One","emails":["one@example.com"]},{"personID":"person-two","displayName":"Lee Two","emails":["two@example.com"]}]}`), nil
			case "http://mattermost.local/api/v4/users?per_page=200":
				return platformDMTestJSONResponse(`[{"id":"user-one","email":"one@example.com","username":"lee"},{"id":"user-two","email":"two@example.com","username":"lee-two"}]`), nil
			case "http://mattermost.local/api/v4/users/me":
				return platformDMTestJSONResponse(`{"id":"bot-user","username":"internkim"}`), nil
			default:
				t.Fatalf("unexpected request %s", request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"lee"},"message":"테스트"}`),
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
	assertPlatformDMRequestCount(t, requestPaths, "http://blueclaw.local/admin/api/policy", 1)
	assertPlatformDMRequestCount(t, requestPaths, "http://mattermost.local/api/v4/users?per_page=200", 1)
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

func platformDMTestService(t *testing.T, tokenPath string, usersDocument string) Service {
	t.Helper()
	return Service{
		Configuration: Configuration{
			BlueclawBaseURL:     "http://blueclaw.local",
			MattermostBaseURL:   "http://mattermost.local",
			MattermostTokenPath: tokenPath,
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch request.URL.String() {
			case "http://blueclaw.local/admin/api/policy":
				return platformDMTestJSONResponse(`{"people":[{"personID":"person-one","displayName":"Lee One","emails":["one@example.com"]},{"personID":"person-two","displayName":"Lee Two","emails":["two@example.com"]},{"personID":"person-gamyeong","displayName":"이샘플","emails":["gamyeong@example.com"]}]}`), nil
			case "http://mattermost.local/api/v4/users?per_page=200":
				return platformDMTestJSONResponse(usersDocument), nil
			case "http://mattermost.local/api/v4/users/me":
				return platformDMTestJSONResponse(`{"id":"bot-user","username":"internkim"}`), nil
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

func writePlatformDMTestFile(t *testing.T, value string) string {
	t.Helper()
	path := t.TempDir() + "/secret"
	if errorValue := os.WriteFile(path, []byte(value), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
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
