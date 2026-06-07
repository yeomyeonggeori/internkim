package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestMattermostChannelUpdateRequiresAdminAndApproval(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://blueclaw.test/admin/api/policy" {
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		}
		t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
	})

	staffResponse, errorValue := service.invokeMattermostTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "mattermost.channel.update",
		Input:    mustJSON(t, map[string]any{"channelID": "channel-1", "header": "hello"}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if staffResponse.Status != "denied" || staffResponse.ErrorCode != capabilities.CapabilityNotAllowed {
		t.Fatalf("expected staff channel update to be denied, got %+v", staffResponse)
	}

	adminResponse, errorValue := service.invokeMattermostTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "mattermost.channel.update",
		Input:    mustJSON(t, map[string]any{"channelID": "channel-1", "header": "hello"}),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "admin@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if adminResponse.Status != "denied" || adminResponse.ErrorCode != "approval_required" {
		t.Fatalf("expected approval requirement, got %+v", adminResponse)
	}
}

func TestMattermostChannelUpdateAdminPatchesAndInvites(t *testing.T) {
	var patchedChannel bool
	var invitedUser bool
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/users/me/channels?per_page=200":
			return testJSONResponse(http.StatusOK, []mattermostToolChannel{{ID: "channel-1", Name: "random", DisplayName: "Random"}}), nil
		case "http://mattermost.test/api/v4/users?per_page=200":
			return testJSONResponse(http.StatusOK, []platformDMMattermostUser{{ID: "alice-1", Email: "alice@example.com", Username: "alice"}}), nil
		case "http://mattermost.test/api/v4/channels/channel-1/patch":
			if request.Method != http.MethodPut {
				t.Fatalf("expected patch method, got %s", request.Method)
			}
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["header"] != "new header" || payload["display_name"] != "Random Room" {
				t.Fatalf("unexpected channel patch: %+v", payload)
			}
			patchedChannel = true
			return testJSONResponse(http.StatusOK, map[string]string{"id": "channel-1"}), nil
		case "http://mattermost.test/api/v4/channels/channel-1/members":
			if request.Method != http.MethodPost {
				t.Fatalf("expected invite method, got %s", request.Method)
			}
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["user_id"] != "alice-1" {
				t.Fatalf("unexpected invite payload: %+v", payload)
			}
			invitedUser = true
			return testJSONResponse(http.StatusCreated, map[string]string{"status": "ok"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokeMattermostTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "mattermost.channel.update",
		Input: mustJSON(t, map[string]any{
			"channelName":  "random",
			"header":       "new header",
			"displayName":  "Random Room",
			"inviteeHints": []string{"alice@example.com"},
		}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "admin@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "updated" || !patchedChannel || !invitedUser {
		t.Fatalf("expected channel update and invite, response=%+v patched=%v invited=%v", response, patchedChannel, invitedUser)
	}
}

func TestMattermostChannelUpdateProtectsManagedHeadersAndDefaultNames(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/channels/flow-channel":
			return testJSONResponse(http.StatusOK, mattermostToolChannel{ID: "flow-channel", Name: "flow", Header: "[업무 열기](/flow/)"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	headerResponse, errorValue := service.invokeMattermostTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "mattermost.channel.update",
		Input:    mustJSON(t, map[string]any{"channelID": "flow-channel", "header": "new header"}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "admin@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if headerResponse.Status != "denied" || headerResponse.ErrorCode != "protected_channel_header" {
		t.Fatalf("expected managed header guardrail, got %+v", headerResponse)
	}

	nameResponse, errorValue := service.invokeMattermostTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "mattermost.channel.update",
		Input:    mustJSON(t, map[string]any{"channelID": "flow-channel", "displayName": "Ops"}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "admin@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if nameResponse.Status != "denied" || nameResponse.ErrorCode != "protected_channel_name" {
		t.Fatalf("expected default name guardrail, got %+v", nameResponse)
	}
}

func TestMattermostChannelPostPinsCreatedPost(t *testing.T) {
	var pinnedPost bool
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/users/me/channels?per_page=200":
			return testJSONResponse(http.StatusOK, []mattermostToolChannel{{ID: "channel-1", Name: "random"}}), nil
		case "http://mattermost.test/api/v4/posts":
			if request.Method != http.MethodPost {
				t.Fatalf("expected post method, got %s", request.Method)
			}
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["channel_id"] != "channel-1" || payload["message"] != "hello" {
				t.Fatalf("unexpected post payload: %+v", payload)
			}
			return testJSONResponse(http.StatusCreated, map[string]string{"id": "post-1"}), nil
		case "http://mattermost.test/api/v4/posts/post-1/pin":
			pinnedPost = true
			return testJSONResponse(http.StatusOK, map[string]string{"status": "ok"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.send",
		Input: mustJSON(t, map[string]any{
			"deliveryTarget": map[string]any{"type": "channel", "channelName": "random"},
			"message":        "hello",
			"pin":            true,
		}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "sent" || !pinnedPost {
		t.Fatalf("expected pinned post, response=%+v pinned=%v", response, pinnedPost)
	}
}

func TestPlatformMessageSearchUsesChannelScope(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/channels/channel-1":
			return testJSONResponse(http.StatusOK, mattermostToolChannel{ID: "channel-1", Name: "random"}), nil
		case "http://mattermost.test/api/v4/channels/channel-1/posts?page=0&per_page=3":
			return testJSONResponse(http.StatusOK, mattermostToolPostsResponse{
				Order: []string{"post-1", "post-2", "post-3"},
				Posts: map[string]mattermostToolPost{
					"post-1": {ID: "post-1", UserID: "bot-1", Message: "one"},
					"post-2": {ID: "post-2", UserID: "user-1", Message: "two"},
					"post-3": {ID: "post-3", UserID: "bot-1", Message: "three"},
				},
			}), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.search",
		Input: mustJSON(t, map[string]any{
			"scope":          "channel",
			"deliveryTarget": map[string]any{"type": "channel", "channelID": "channel-1"},
			"authoredBy":     "assistant",
			"limit":          3,
		}),
		Context: capabilities.ToolInvokeContext{RequesterEmail: "staff@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" {
		t.Fatalf("expected search success, got %+v", response)
	}
	var result struct {
		CandidateCount int `json:"candidateCount"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.CandidateCount != 2 {
		t.Fatalf("unexpected search result: %+v", result)
	}
}

func TestMattermostPostUpdateAndDeleteGuardrails(t *testing.T) {
	var botPostDeleted bool
	var userPostPinned bool
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case request.URL.String() == "http://mattermost.test/api/v4/posts/user-post":
			return testJSONResponse(http.StatusOK, mattermostToolPost{ID: "user-post", UserID: "user-1"}), nil
		case request.URL.String() == "http://mattermost.test/api/v4/posts/bot-post" && request.Method == http.MethodGet:
			return testJSONResponse(http.StatusOK, mattermostToolPost{ID: "bot-post", UserID: "bot-1"}), nil
		case request.URL.String() == "http://mattermost.test/api/v4/posts/protected-post":
			return testJSONResponse(http.StatusOK, mattermostToolPost{ID: "protected-post", UserID: "bot-1", Props: map[string]any{"internkim_calendar_event": true}}), nil
		case request.URL.String() == "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim"}), nil
		case request.URL.String() == "http://mattermost.test/api/v4/posts/user-post/pin":
			userPostPinned = true
			return testJSONResponse(http.StatusOK, map[string]string{"status": "ok"}), nil
		case request.URL.String() == "http://mattermost.test/api/v4/posts/bot-post" && request.Method == http.MethodDelete:
			botPostDeleted = true
			return testJSONResponse(http.StatusOK, map[string]string{"status": "ok"}), nil
		default:
			if strings.HasSuffix(request.URL.String(), "/patch") {
				t.Fatalf("unexpected patch request: %s", request.URL.String())
			}
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	messageResponse, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.update",
		Input:    mustJSON(t, map[string]any{"messageID": "user-post", "message": "changed"}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if messageResponse.Status != "denied" || messageResponse.ErrorCode != "not_bot_post" {
		t.Fatalf("expected non-bot message update denial, got %+v", messageResponse)
	}

	pinResponse, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.update",
		Input:    mustJSON(t, map[string]any{"messageID": "user-post", "isPinned": true}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if pinResponse.Status != "updated" || !userPostPinned {
		t.Fatalf("expected user post pin, response=%+v pinned=%v", pinResponse, userPostPinned)
	}

	protectedResponse, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.update",
		Input:    mustJSON(t, map[string]any{"messageID": "protected-post", "isPinned": false}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if protectedResponse.Status != "denied" || protectedResponse.ErrorCode != "protected_post" {
		t.Fatalf("expected protected post denial, got %+v", protectedResponse)
	}

	deleteResponse, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.delete",
		Input:    mustJSON(t, map[string]any{"messageIDs": []string{"bot-post"}}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if deleteResponse.Status != "deleted" || !botPostDeleted {
		t.Fatalf("expected bot post delete, response=%+v deleted=%v", deleteResponse, botPostDeleted)
	}
}

func TestMattermostPostDeleteDeletesMultiplePostsAndReportsFailures(t *testing.T) {
	deletedPostIDs := []string{}
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case request.URL.String() == "http://mattermost.test/api/v4/posts/bot-post-1" && request.Method == http.MethodGet:
			return testJSONResponse(http.StatusOK, mattermostToolPost{ID: "bot-post-1", UserID: "bot-1"}), nil
		case request.URL.String() == "http://mattermost.test/api/v4/posts/bot-post-2" && request.Method == http.MethodGet:
			return testJSONResponse(http.StatusOK, mattermostToolPost{ID: "bot-post-2", UserID: "bot-1"}), nil
		case request.URL.String() == "http://mattermost.test/api/v4/posts/user-post" && request.Method == http.MethodGet:
			return testJSONResponse(http.StatusOK, mattermostToolPost{ID: "user-post", UserID: "user-1"}), nil
		case request.URL.String() == "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim"}), nil
		case strings.HasPrefix(request.URL.String(), "http://mattermost.test/api/v4/posts/bot-post-") && request.Method == http.MethodDelete:
			deletedPostIDs = append(deletedPostIDs, strings.TrimPrefix(request.URL.Path, "/api/v4/posts/"))
			return testJSONResponse(http.StatusOK, map[string]string{"status": "ok"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.delete",
		Input:    mustJSON(t, map[string]any{"messageIDs": []string{"bot-post-1", "user-post", "bot-post-2"}}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "deleted" || len(deletedPostIDs) != 2 {
		t.Fatalf("expected partial delete success, response=%+v deleted=%+v", response, deletedPostIDs)
	}
	var result struct {
		DeletedCount int `json:"deletedCount"`
		FailedCount  int `json:"failedCount"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.DeletedCount != 2 || result.FailedCount != 1 {
		t.Fatalf("unexpected delete result: %+v", result)
	}
}

func TestMattermostPostDeleteFailsWhenNothingWasDeleted(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/posts/user-post":
			return testJSONResponse(http.StatusOK, mattermostToolPost{ID: "user-post", UserID: "user-1"}), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.delete",
		Input:    mustJSON(t, map[string]any{"messageIDs": []string{"user-post"}}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "post_delete_not_completed" {
		t.Fatalf("expected delete failure, got %+v", response)
	}
}

func TestMattermostContextInspectReturnsCurrentMattermostContext(t *testing.T) {
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "mattermost", ConversationID: "thread:channel-1:root-1", ChannelID: "channel-1", ChannelType: "O", RootID: "root-1", MessageID: "post-1"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://blueclaw.test/admin/api/policy" {
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		}
		if request.URL.String() == "http://mattermost.test/api/v4/users/me" {
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim"}), nil
		}
		t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.context",
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			ConversationID: "thread:channel-1:root-1",
			ChannelID:      "channel-1",
			ChannelName:    "town-square",
			ReplyTargetID:  replyTargetID,
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" {
		t.Fatalf("expected inspect success, got %+v", response)
	}
}

func TestMattermostPostSearchUsesCurrentThreadScope(t *testing.T) {
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "mattermost", ConversationID: "thread:channel-1:root-1", ChannelID: "channel-1", ChannelType: "O", RootID: "root-1"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/posts/root-1/thread":
			return testJSONResponse(http.StatusOK, mattermostToolPostsResponse{
				Order: []string{"bot-post", "user-post"},
				Posts: map[string]mattermostToolPost{
					"bot-post":  {ID: "bot-post", UserID: "bot-1", ChannelID: "channel-1", RootID: "root-1", Message: "테스트 메시지", CreateAt: 1},
					"user-post": {ID: "user-post", UserID: "user-1", ChannelID: "channel-1", RootID: "root-1", Message: "사용자 메시지", CreateAt: 2},
				},
			}), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.search",
		Input:    mustJSON(t, map[string]any{"scope": "currentThread", "authoredBy": "assistant"}),
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			ConversationID: "thread:channel-1:root-1",
			ChannelID:      "channel-1",
			ReplyTargetID:  replyTargetID,
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result struct {
		CandidateCount int `json:"candidateCount"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" || result.CandidateCount != 1 {
		t.Fatalf("expected one bot candidate, response=%+v result=%+v", response, result)
	}
}

func TestMattermostPostSearchUsesDirectMessageScope(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/users?per_page=200":
			return testJSONResponse(http.StatusOK, []platformDMMattermostUser{{ID: "alice-1", Email: "alice@example.com", Username: "alice"}}), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim"}), nil
		case "http://mattermost.test/api/v4/users/me/channels?per_page=200":
			return testJSONResponse(http.StatusOK, []mattermostToolChannel{{ID: "dm-1", Name: "alice-1__bot-1", Type: "D"}}), nil
		case "http://mattermost.test/api/v4/channels/dm-1/posts?page=0&per_page=50":
			return testJSONResponse(http.StatusOK, mattermostToolPostsResponse{
				Order: []string{"bot-dm-post"},
				Posts: map[string]mattermostToolPost{
					"bot-dm-post": {ID: "bot-dm-post", UserID: "bot-1", ChannelID: "dm-1", Message: "안녕", CreateAt: 1},
				},
			}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "platform.message.search",
		Input: mustJSON(t, map[string]any{
			"scope":          "directMessage",
			"deliveryTarget": map[string]any{"type": "directMessage", "personHint": "alice@example.com"},
			"authoredBy":     "assistant",
		}),
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result struct {
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
		CandidateCount int `json:"candidateCount"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" || result.Channel.ID != "dm-1" || result.CandidateCount != 1 {
		t.Fatalf("expected direct message candidate, response=%+v result=%+v", response, result)
	}
}

func mattermostToolTestService(t *testing.T, roundTrip func(*http.Request) (*http.Response, error)) Service {
	t.Helper()
	return Service{
		Configuration: Configuration{
			MattermostBaseURL:   "http://mattermost.test",
			MattermostTokenPath: writePlatformTestFile(t, "mattermost-token"),
			BlueclawBaseURL:     "http://blueclaw.test",
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(roundTrip)},
	}
}

func mattermostToolTestPolicy() mattermostToolPolicyDocument {
	return mattermostToolPolicyDocument{
		People: []mattermostToolPolicyPerson{
			{
				PersonID: "admin-1",
				Emails:   []string{"admin@example.com"},
				Circles:  []string{"staff", "admin"},
				IsAdmin:  true,
			},
			{
				PersonID: "staff-1",
				Emails:   []string{"staff@example.com"},
				Circles:  []string{"staff"},
			},
			{
				PersonID: "alice-1",
				Emails:   []string{"alice@example.com"},
				Circles:  []string{"staff"},
			},
		},
	}
}
