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

const platformDMResolveEndpoint = "http://blueclaw.local/admin/api/identity/resolve-recipient"

const platformDMResolvedDongha = `{"status":"resolved","recipient":{"personID":"person-dongha","displayName":"이동하","emails":["dongha@example.com"],"externalUserID":"user-dongha"}}`

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
			case platformDMResolveEndpoint:
				return platformDMTestJSONResponse(platformDMResolvedDongha), nil
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
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"동하"},"message":"테스트"}`),
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

func TestPlatformMessageSendRecipientHintSendsMattermostDM(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, platformDMResolvedDongha)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"recipientHint":"동하","message":"테스트"}`),
		Context: capabilities.ToolInvokeContext{
			IsScheduledRun: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "sent" || response.IsError {
		t.Fatalf("expected recipientHint DM to send, got %+v", response)
	}
}

func TestPlatformMessageSendRejectsConflictingRecipientHint(t *testing.T) {
	response, errorValue := Service{}.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"동하"},"recipientHint":"다른 사람","message":"테스트"}`),
		Context:  capabilities.ToolInvokeContext{IsScheduledRun: true},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "error" || response.ErrorCode != "invalid_input" {
		t.Fatalf("expected invalid input conflict, got %+v", response)
	}
	if !strings.Contains(response.Content, "recipientHint conflicts") {
		t.Fatalf("expected conflict message, got %+v", response)
	}
}

func TestPlatformDMSendImmediateRunRequiresApprovalContext(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, platformDMResolvedDongha)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"동하"},"message":"테스트"}`),
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
	service := platformDMTestService(t, tokenPath, platformDMResolvedDongha)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"동하"},"message":"본인 확인"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterPersonID:       "person-dongha",
			RequesterPlatformUserID: "user-dongha",
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
	service := platformDMTestService(t, tokenPath, platformDMResolvedDongha)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"@dongha"},"message":"승인 후 전송"}`),
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

func TestPlatformDMSendUnlinkedRecipientFallsBackToEmailLookup(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	unlinkedResolution := `{"status":"unlinked","recipient":{"personID":"person-dongha","displayName":"이동하","emails":["dongha@example.com"]}}`
	service := platformDMTestService(t, tokenPath, unlinkedResolution)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"동하"},"message":"테스트"}`),
		Context: capabilities.ToolInvokeContext{
			IsScheduledRun: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "sent" || response.IsError {
		t.Fatalf("expected unlinked fallback to send, got %+v", response)
	}
}

func TestPlatformDMSendUnlinkedRecipientWithoutAccountFails(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	unlinkedResolution := `{"status":"unlinked","recipient":{"personID":"person-ghost","displayName":"유령","emails":["ghost@example.com"]}}`
	service := Service{
		Configuration: Configuration{
			BlueclawBaseURL:     "http://blueclaw.local",
			MattermostBaseURL:   "http://mattermost.local",
			MattermostTokenPath: tokenPath,
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch request.URL.String() {
			case platformDMResolveEndpoint:
				return platformDMTestJSONResponse(unlinkedResolution), nil
			case "http://mattermost.local/api/v4/users/email/ghost@example.com":
				return platformDMTestStatusResponse(http.StatusNotFound, "not found"), nil
			default:
				t.Fatalf("unexpected request %s", request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"유령"},"message":"테스트"}`),
		Context:  capabilities.ToolInvokeContext{IsScheduledRun: true},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertPlatformDMStructuredFailure(t, response, "error", "recipient_not_found", "recipient_resolve", false, false)
}

func TestPlatformDMSendNotFoundCarriesApprovedPeople(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	notFoundResolution := `{"status":"not_found","approvedPeople":["이동하","신우경"]}`
	service := platformDMTestService(t, tokenPath, notFoundResolution)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"없는사람"},"message":"테스트"}`),
		Context:  capabilities.ToolInvokeContext{IsScheduledRun: true},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertPlatformDMStructuredFailure(t, response, "error", "recipient_not_found", "recipient_resolve", false, false)
	var failure platformDMFailure
	if errorValue := json.Unmarshal(response.Result, &failure); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(failure.ApprovedPeople) != 2 {
		t.Fatalf("expected approved people directory, got %+v", failure)
	}
	if !strings.Contains(response.Message, "신우경") {
		t.Fatalf("expected approved people in message, got %+v", response.Message)
	}
}

func TestPlatformDMSendIdentityUnavailableIsRetryable(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := Service{
		Configuration: Configuration{
			BlueclawBaseURL:     "http://blueclaw.local",
			MattermostBaseURL:   "http://mattermost.local",
			MattermostTokenPath: tokenPath,
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch request.URL.String() {
			case platformDMResolveEndpoint:
				return platformDMTestStatusResponse(http.StatusServiceUnavailable, "blueclaw unavailable"), nil
			default:
				t.Fatalf("unexpected request %s", request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"동하"},"message":"테스트"}`),
		Context:  capabilities.ToolInvokeContext{IsScheduledRun: true},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertPlatformDMStructuredFailure(t, response, "error", "identity_unavailable", "recipient_resolve", true, true)
}

func TestPlatformDMSendMissingMattermostTokenDoesNotSend(t *testing.T) {
	service := platformDMTestService(t, "/missing/token", platformDMResolvedDongha)

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"동하"},"message":"테스트"}`),
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
			case platformDMResolveEndpoint:
				return platformDMTestJSONResponse(platformDMResolvedDongha), nil
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
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"동하"},"message":"테스트"}`),
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
			case platformDMResolveEndpoint:
				return platformDMTestJSONResponse(platformDMResolvedDongha), nil
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
		Input:    []byte(`{"deliveryTarget":{"type":"directMessage","personHint":"동하"},"message":"테스트"}`),
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
	ambiguousResolution := `{"status":"ambiguous","candidates":[{"personID":"person-one","displayName":"Lee One","emails":["one@example.com"],"externalUserID":"user-one"},{"personID":"person-two","displayName":"Lee Two","emails":["two@example.com"],"externalUserID":"user-two"}]}`
	service := Service{
		Configuration: Configuration{
			BlueclawBaseURL:     "http://blueclaw.local",
			MattermostBaseURL:   "http://mattermost.local",
			MattermostTokenPath: tokenPath,
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestPaths = append(requestPaths, request.URL.String())
			switch request.URL.String() {
			case platformDMResolveEndpoint:
				return platformDMTestJSONResponse(ambiguousResolution), nil
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
	assertPlatformDMRequestCount(t, requestPaths, platformDMResolveEndpoint, 1)
	if len(requestPaths) != 1 {
		t.Fatalf("ambiguous resolution made extra requests: %+v", requestPaths)
	}
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
			case platformDMResolveEndpoint:
				return platformDMTestJSONResponse(resolutionDocument), nil
			case "http://mattermost.local/api/v4/users/email/dongha@example.com":
				return platformDMTestJSONResponse(`{"id":"user-dongha","email":"dongha@example.com","username":"dongha"}`), nil
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
