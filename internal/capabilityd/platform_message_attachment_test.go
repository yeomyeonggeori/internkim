package capabilityd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func platformMessageAttachmentTestService(t *testing.T, workspacePath string, roundTrip func(*http.Request) (*http.Response, error)) Service {
	t.Helper()
	service := mattermostToolTestService(t, roundTrip)
	service.Configuration.BlueclawWorkspacePath = workspacePath
	return service
}

func writeWorkspaceAttachment(t *testing.T, workspacePath string, relativePath string) string {
	t.Helper()
	hostPath := filepath.Join(workspacePath, relativePath)
	if errorValue := os.MkdirAll(filepath.Dir(hostPath), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(hostPath, []byte("\x89PNG\r\n\x1a\nimage-bytes"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	return "/workspace/" + relativePath
}

func TestPlatformMessageSendUploadsWorkspaceAttachmentToUnjoinedChannel(t *testing.T) {
	workspacePath := t.TempDir()
	agentPath := writeWorkspaceAttachment(t, workspacePath, "shared/inbox/notice.png")
	var botJoinedChannel bool
	var uploadedFile bool
	var postedFileIDs []any
	service := platformMessageAttachmentTestService(t, workspacePath, func(request *http.Request) (*http.Response, error) {
		if isDirectoryPeopleRequest(request) {
			return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
		}
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/users/me/channels?per_page=200":
			return testJSONResponse(http.StatusOK, []mattermostToolChannel{}), nil
		case "http://mattermost.test/api/v4/users/me/teams":
			return testJSONResponse(http.StatusOK, []map[string]string{{"id": "team-1"}}), nil
		case "http://mattermost.test/api/v4/teams/team-1/channels/search":
			return testJSONResponse(http.StatusOK, []mattermostToolChannel{{ID: "channel-9", Name: "off-topic", DisplayName: "잡담"}}), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		case "http://mattermost.test/api/v4/channels/channel-9/members/bot-1":
			return testJSONResponse(http.StatusNotFound, map[string]string{"message": "no channel member"}), nil
		case "http://mattermost.test/api/v4/channels/channel-9/members":
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["user_id"] != "bot-1" {
				t.Fatalf("unexpected join payload: %+v", payload)
			}
			botJoinedChannel = true
			return testJSONResponse(http.StatusCreated, map[string]string{"channel_id": "channel-9", "user_id": "bot-1"}), nil
		case "http://mattermost.test/api/v4/channels/channel-9/members/staff-1":
			if !botJoinedChannel {
				return testJSONResponse(http.StatusForbidden, map[string]string{"message": "permissions to the channel"}), nil
			}
			return testJSONResponse(http.StatusOK, map[string]string{"channel_id": "channel-9", "user_id": "staff-1"}), nil
		case "http://mattermost.test/api/v4/files":
			uploadedFile = true
			return testJSONResponse(http.StatusCreated, map[string]any{"file_infos": []map[string]string{{"id": "file-7"}}}), nil
		case "http://mattermost.test/api/v4/posts":
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			fileIDs, _ := payload["file_ids"].([]any)
			postedFileIDs = fileIDs
			if payload["channel_id"] != "channel-9" {
				t.Fatalf("unexpected post payload: %+v", payload)
			}
			return testJSONResponse(http.StatusCreated, map[string]string{"id": "post-9"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input: mustJSON(t, map[string]any{
			"targetType":  "channel",
			"channelName": "잡담",
			"message":     "번역본입니다",
			"attachments": []string{agentPath},
		}),
		Transport: capabilities.ToolInvokeTransport{
			WorkspaceFiles: []capabilities.WorkspaceFile{{
				WorkspacePath: agentPath,
				Filename:      filepath.Base(agentPath),
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("a picture")),
			}},
		},
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:          "staff@example.com",
			RequesterPlatformUserID: "staff-1",
			IsApprovalContinuation:  true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "sent" || !botJoinedChannel || !uploadedFile {
		t.Fatalf("expected joined upload send, response=%+v joined=%v uploaded=%v", response, botJoinedChannel, uploadedFile)
	}
	if len(postedFileIDs) != 1 || postedFileIDs[0] != "file-7" {
		t.Fatalf("expected post to carry uploaded file, got %+v", postedFileIDs)
	}
}

func TestPlatformMessageSendMembershipLookupErrorIsNotAccessDenial(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		if isDirectoryPeopleRequest(request) {
			return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
		}
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/channels/channel-1":
			return testJSONResponse(http.StatusOK, mattermostToolChannel{ID: "channel-1", Name: "random"}), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		case "http://mattermost.test/api/v4/channels/channel-1/members/bot-1":
			return testJSONResponse(http.StatusOK, map[string]string{"channel_id": "channel-1", "user_id": "bot-1"}), nil
		case "http://mattermost.test/api/v4/channels/channel-1/members/staff-1":
			return testJSONResponse(http.StatusInternalServerError, map[string]string{"message": "database is unavailable"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input: mustJSON(t, map[string]any{
			"targetType": "channel",
			"channelID":  "channel-1",
			"message":    "hello",
		}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:          "staff@example.com",
			RequesterPlatformUserID: "staff-1",
			IsApprovalContinuation:  true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.ErrorCode != "mattermost_unavailable" {
		t.Fatalf("expected lookup error to surface as mattermost_unavailable, got %+v", response)
	}
}

func TestPlatformMessageSendDoesNotJoinChannelForNonMemberRequester(t *testing.T) {
	var botJoinAttempted bool
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		if isDirectoryPeopleRequest(request) {
			return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
		}
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/channels/channel-1":
			return testJSONResponse(http.StatusOK, mattermostToolChannel{ID: "channel-1", Name: "random"}), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		case "http://mattermost.test/api/v4/channels/channel-1/members/staff-1":
			return testJSONResponse(http.StatusNotFound, map[string]string{"message": "no channel member"}), nil
		case "http://mattermost.test/api/v4/channels/channel-1/members":
			botJoinAttempted = true
			return testJSONResponse(http.StatusCreated, map[string]string{"channel_id": "channel-1", "user_id": "bot-1"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input: mustJSON(t, map[string]any{
			"targetType": "channel",
			"channelID":  "channel-1",
			"message":    "hello",
		}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:          "staff@example.com",
			RequesterPlatformUserID: "staff-1",
			IsApprovalContinuation:  true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.ErrorCode != "channel_access_not_authorized" {
		t.Fatalf("expected access denial, got %+v", response)
	}
	if botJoinAttempted {
		t.Fatal("expected no bot channel join for a non-member requester")
	}
}

// A file is read by the person sending it, where their identity exists, and
// arrives here as content. Naming one is not enough: this daemon runs as root
// beside the workspace and opening it on somebody's behalf is the thing being
// taken away, so a named-but-uncarried file is refused.
func TestPlatformMessageSendRefusesAFileNobodyCarried(t *testing.T) {
	service := platformMessageAttachmentTestService(t, t.TempDir(), func(request *http.Request) (*http.Response, error) {
		if isDirectoryPeopleRequest(request) {
			return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
		}
		if request.URL.String() == "http://blueclaw.test/admin/api/policy" {
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		}
		t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input: mustJSON(t, map[string]any{
			"targetType":  "channel",
			"channelID":   "channel-1",
			"message":     "hello",
			"attachments": []string{"/workspace/shared/missing.png"},
		}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:          "staff@example.com",
			RequesterPlatformUserID: "staff-1",
			IsApprovalContinuation:  true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.ErrorCode != "attachment_not_carried" {
		t.Fatalf("expected attachment_not_carried, got %+v", response)
	}
}
