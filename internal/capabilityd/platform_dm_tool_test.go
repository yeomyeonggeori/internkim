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

	response, errorValue := service.invokePlatformDMSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.dm.send",
		Input:    []byte(`{"recipientHint":"샘플","message":"테스트"}`),
		Context: capabilities.ToolInvokeContext{
			IsScheduledRun: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" || response.IsError {
		t.Fatalf("expected ok response, got %+v", response)
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

	response, errorValue := service.invokePlatformDMSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.dm.send",
		Input:    []byte(`{"recipientHint":"샘플","message":"테스트"}`),
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
}

func TestPlatformDMSendImmediateSelfDoesNotRequireApproval(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, `[{"id":"user-gamyeong","email":"gamyeong@example.com","username":"gamyeong","nickname":"샘플"}]`)

	response, errorValue := service.invokePlatformDMSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.dm.send",
		Input:    []byte(`{"recipientHint":"샘플","message":"본인 확인"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterPersonID:       "person-gamyeong",
			RequesterPlatformUserID: "user-gamyeong",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" || response.IsError {
		t.Fatalf("expected self DM to send without approval, got %+v", response)
	}
}

func TestPlatformDMSendApprovedContinuationSendsMattermostDM(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, `[{"id":"user-gamyeong","email":"gamyeong@example.com","username":"gamyeong"}]`)

	response, errorValue := service.invokePlatformDMSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.dm.send",
		Input:    []byte(`{"recipientHint":"@gamyeong","message":"승인 후 전송"}`),
		Context: capabilities.ToolInvokeContext{
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" || response.IsError {
		t.Fatalf("expected approved continuation to send, got %+v", response)
	}
}

func TestPlatformDMSendMatchesMattermostNickname(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, `[{"id":"user-gamyeong","email":"gamyeong@example.com","username":"member-42","nickname":"샘플"}]`)

	response, errorValue := service.invokePlatformDMSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.dm.send",
		Input:    []byte(`{"recipientHint":"샘플","message":"테스트"}`),
		Context: capabilities.ToolInvokeContext{
			IsScheduledRun: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" || response.IsError {
		t.Fatalf("expected Mattermost nickname match, got %+v", response)
	}
}

func TestPlatformDMSendAmbiguousRecipientDoesNotSend(t *testing.T) {
	tokenPath := writePlatformDMTestFile(t, "bot-token")
	service := platformDMTestService(t, tokenPath, `[
		{"id":"user-one","email":"one@example.com","username":"lee"},
		{"id":"user-two","email":"two@example.com","username":"lee-two"}
	]`)

	response, errorValue := service.invokePlatformDMSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.dm.send",
		Input:    []byte(`{"recipientHint":"lee","message":"테스트"}`),
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
}

func TestPlatformDMSendMissingMattermostTokenDoesNotSend(t *testing.T) {
	service := platformDMTestService(t, "/missing/token", `[{"id":"user-gamyeong","email":"gamyeong@example.com","username":"gamyeong"}]`)

	response, errorValue := service.invokePlatformDMSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.dm.send",
		Input:    []byte(`{"recipientHint":"샘플","message":"테스트"}`),
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
