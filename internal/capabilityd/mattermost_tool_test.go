package capabilityd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestMattermostChannelUpdateRequiresAdminAfterApproval(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://blueclaw.test/admin/api/policy" {
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		}
		t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
	})

	staffResponse, errorValue := service.invokeMattermostTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "channel.update",
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
		case "http://blueclaw.test/admin/api/identity/resolve-recipient":
			return testJSONResponse(http.StatusOK, map[string]any{
				"status": "resolved",
				"recipient": map[string]any{
					"personID":       "person-alice",
					"displayName":    "Alice",
					"emails":         []string{"alice@example.com"},
					"externalUserID": "alice-1",
					"username":       "alice",
				},
			}), nil
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
		ToolName: "channel.update",
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
	if response.Status != "updated" || response.Outcome != capabilities.ToolOutcomeSucceeded || len(response.Effects) != 1 || response.Effects[0].ID != "channel-1" || !patchedChannel || !invitedUser {
		t.Fatalf("expected channel update and invite, response=%+v patched=%v invited=%v", response, patchedChannel, invitedUser)
	}
	var result mattermostChannelUpdateResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !reflect.DeepEqual(result.InvitedUserIDs, []string{"alice-1"}) {
		t.Fatalf("unexpected canonical channel result: %+v", result)
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
		ToolName: "channel.update",
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
		ToolName: "channel.update",
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
		case "http://mattermost.test/api/v4/channels/channel-1/members/staff-1":
			return testJSONResponse(http.StatusOK, map[string]string{"channel_id": "channel-1", "user_id": "staff-1"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.send",
		Input: mustJSON(t, map[string]any{
			"targetType": "channel", "channelName": "random",
			"message": "hello",
			"pin":     true,
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
		case "http://mattermost.test/api/v4/channels/channel-1/posts?page=0&per_page=100":
			return testJSONResponse(http.StatusOK, mattermostToolPostsResponse{
				Order: []string{"post-1", "post-2", "post-3"},
				Posts: map[string]mattermostToolPost{
					"post-1": {ID: "post-1", UserID: "bot-1", Message: "one"},
					"post-2": {ID: "post-2", UserID: "user-1", Message: "two"},
					"post-3": {ID: "post-3", UserID: "bot-1", Message: "three"},
				},
			}), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		case "http://mattermost.test/api/v4/channels/channel-1/members/staff-1":
			return testJSONResponse(http.StatusOK, map[string]string{"channel_id": "channel-1", "user_id": "staff-1"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.search",
		Input: mustJSON(t, map[string]any{
			"scope":      "channel",
			"channelID":  "channel-1",
			"authoredBy": "assistant",
			"limit":      3,
		}),
		Context: capabilities.ToolInvokeContext{RequesterEmail: "staff@example.com", RequesterPlatformUserID: "staff-1"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" || response.Outcome != capabilities.ToolOutcomeSucceeded || response.Effects == nil || len(response.Effects) != 0 {
		t.Fatalf("expected search success, got %+v", response)
	}
	var result platformMessageSearchResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("unexpected search result: %+v", result)
	}
}

func TestPlatformMessageSearchMatchesAnyQuery(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/channels/channel-1/posts?page=0&per_page=100":
			return testJSONResponse(http.StatusOK, mattermostToolPostsResponse{
				Order: []string{"sky-post", "night-post", "other-post"},
				Posts: map[string]mattermostToolPost{
					"sky-post":   {ID: "sky-post", UserID: "bot-1", ChannelID: "channel-1", Message: "하늘 아래 남긴 문장", CreateAt: 1},
					"night-post": {ID: "night-post", UserID: "bot-1", ChannelID: "channel-1", Message: "밤에 보낸 문장", CreateAt: 2},
					"other-post": {ID: "other-post", UserID: "bot-1", ChannelID: "channel-1", Message: "다른 문장", CreateAt: 3},
				},
			}), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.search",
		Input: mustJSON(t, map[string]any{
			"scope":      "currentChannel",
			"authoredBy": "assistant",
			"queries":    []string{"하늘", "밤"},
		}),
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			ConversationID: "channel:channel-1",
			ChannelID:      "channel-1",
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result struct {
		MessageIDs []string `json:"messageIDs"`
		Queries    []string `json:"queries"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" || !reflect.DeepEqual(result.MessageIDs, []string{"sky-post", "night-post"}) {
		t.Fatalf("expected OR search to return matching bot posts, response=%+v result=%+v", response, result)
	}
	if !reflect.DeepEqual(result.Queries, []string{"하늘", "밤"}) {
		t.Fatalf("expected queries to round trip, got %+v", result.Queries)
	}
}

func TestPlatformMessageSearchPaginatesCandidates(t *testing.T) {
	posts := mattermostToolPostsResponse{Order: []string{}, Posts: map[string]mattermostToolPost{}}
	for index := 1; index <= 105; index++ {
		postID := fmt.Sprintf("bot-post-%02d", index)
		posts.Order = append(posts.Order, postID)
		posts.Posts[postID] = mattermostToolPost{ID: postID, UserID: "bot-1", ChannelID: "channel-1", Message: "삭제대상", CreateAt: int64(index)}
	}
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/channels/channel-1/posts?page=1&per_page=100":
			page := mattermostToolPostsResponse{Order: posts.Order[100:], Posts: posts.Posts}
			return testJSONResponse(http.StatusOK, page), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.search",
		Input: mustJSON(t, map[string]any{
			"scope":      "currentChannel",
			"authoredBy": "assistant",
			"queries":    []string{"삭제대상"},
			"cursor":     "1",
		}),
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			ConversationID: "channel:channel-1",
			ChannelID:      "channel-1",
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result platformMessageSearchResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.HasMore || result.NextCursor != "" {
		t.Fatalf("unexpected pagination state: %+v", result)
	}
	if len(result.Candidates) != 5 {
		t.Fatalf("unexpected candidate counts: %+v", result)
	}
	if result.Candidates[0].MessageID != "bot-post-101" {
		t.Fatalf("expected second page candidates with exact IDs, got %+v", result.Candidates)
	}
	if len(result.MessageIDs) != 5 || result.MessageIDs[0] != "bot-post-101" {
		t.Fatalf("expected compact candidate ids before previews, got %+v", result.MessageIDs)
	}
}

func TestPlatformMessageSearchScansUntilLimitMatches(t *testing.T) {
	firstPage := mattermostToolPostsResponse{Order: []string{}, Posts: map[string]mattermostToolPost{}}
	for index := 1; index <= 100; index++ {
		postID := fmt.Sprintf("other-post-%02d", index)
		firstPage.Order = append(firstPage.Order, postID)
		firstPage.Posts[postID] = mattermostToolPost{ID: postID, UserID: "bot-1", ChannelID: "channel-1", Message: "다른 내용", CreateAt: int64(index)}
	}
	secondPage := mattermostToolPostsResponse{Order: []string{}, Posts: map[string]mattermostToolPost{}}
	for index := 1; index <= 5; index++ {
		postID := fmt.Sprintf("match-post-%02d", index)
		secondPage.Order = append(secondPage.Order, postID)
		secondPage.Posts[postID] = mattermostToolPost{ID: postID, UserID: "bot-1", ChannelID: "channel-1", Message: "삭제대상", CreateAt: int64(100 + index)}
	}
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/channels/channel-1/posts?page=0&per_page=100":
			return testJSONResponse(http.StatusOK, firstPage), nil
		case "http://mattermost.test/api/v4/channels/channel-1/posts?page=1&per_page=100":
			return testJSONResponse(http.StatusOK, secondPage), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.search",
		Input: mustJSON(t, map[string]any{
			"scope":      "currentChannel",
			"authoredBy": "assistant",
			"queries":    []string{"삭제대상"},
			"limit":      5,
		}),
		Context: capabilities.ToolInvokeContext{
			Platform:       "mattermost",
			ConversationID: "channel:channel-1",
			ChannelID:      "channel-1",
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result platformMessageSearchResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.HasMore || len(result.Candidates) != 5 || !reflect.DeepEqual(result.MessageIDs, []string{"match-post-01", "match-post-02", "match-post-03", "match-post-04", "match-post-05"}) {
		t.Fatalf("expected search to scan past non-matching latest page, got %+v", result)
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
		case request.URL.String() == "http://mattermost.test/api/v4/posts/protected-flow-post":
			return testJSONResponse(http.StatusOK, mattermostToolPost{ID: "protected-flow-post", UserID: "bot-1", Props: map[string]any{"internkim_flow_task": true}}), nil
		case request.URL.String() == "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
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
		ToolName: "message.update",
		Input:    mustJSON(t, map[string]any{"messageID": "user-post", "message": "changed"}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if messageResponse.Status != "denied" || messageResponse.Outcome != capabilities.ToolOutcomeDenied || messageResponse.ErrorCode != "not_bot_post" {
		t.Fatalf("expected non-bot message update denial, got %+v", messageResponse)
	}

	pinResponse, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.update",
		Input:    mustJSON(t, map[string]any{"messageID": "user-post", "isPinned": true}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if pinResponse.Status != "updated" || pinResponse.Outcome != capabilities.ToolOutcomeSucceeded || len(pinResponse.Effects) != 1 || pinResponse.Effects[0].ID != "user-post" || !userPostPinned {
		t.Fatalf("expected user post pin, response=%+v pinned=%v", pinResponse, userPostPinned)
	}

	protectedResponse, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.update",
		Input:    mustJSON(t, map[string]any{"messageID": "protected-post", "isPinned": false}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if protectedResponse.Status != "denied" || protectedResponse.Outcome != capabilities.ToolOutcomeDenied || protectedResponse.ErrorCode != "protected_post" {
		t.Fatalf("expected protected post denial, got %+v", protectedResponse)
	}

	protectedFlowResponse, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.delete",
		Input:    mustJSON(t, map[string]any{"messageIDs": []string{"protected-flow-post"}}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if protectedFlowResponse.Status != "error" || protectedFlowResponse.Outcome != capabilities.ToolOutcomeFailed || len(protectedFlowResponse.Effects) != 0 || protectedFlowResponse.ErrorCode != "post_delete_not_completed" {
		t.Fatalf("expected protected Flow post delete denial, got %+v", protectedFlowResponse)
	}

	deleteResponse, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.delete",
		Input:    mustJSON(t, map[string]any{"messageIDs": []string{"bot-post"}}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if deleteResponse.Status != "deleted" || deleteResponse.Outcome != capabilities.ToolOutcomeSucceeded || len(deleteResponse.Effects) != 1 || deleteResponse.Effects[0].ID != "bot-post" || !botPostDeleted {
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
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		case strings.HasPrefix(request.URL.String(), "http://mattermost.test/api/v4/posts/bot-post-") && request.Method == http.MethodDelete:
			deletedPostIDs = append(deletedPostIDs, strings.TrimPrefix(request.URL.Path, "/api/v4/posts/"))
			return testJSONResponse(http.StatusOK, map[string]string{"status": "ok"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.delete",
		Input:    mustJSON(t, map[string]any{"messageIDs": []string{"bot-post-1", "user-post", "bot-post-2"}}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "deleted" || response.Outcome != capabilities.ToolOutcomeSucceeded || len(response.Effects) != 2 || len(deletedPostIDs) != 2 {
		t.Fatalf("expected partial delete success, response=%+v deleted=%+v", response, deletedPostIDs)
	}
	var result platformMessageDeleteResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(result.MessageIDs) != 2 || len(result.Failures) != 1 {
		t.Fatalf("unexpected delete result: %+v", result)
	}
}

func TestPlatformMessageDeleteRejectsTooManyExactIDs(t *testing.T) {
	messageIDs := []string{}
	for index := 1; index <= 26; index++ {
		messageIDs = append(messageIDs, fmt.Sprintf("post-%02d", index))
	}
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://blueclaw.test/admin/api/policy" {
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		}
		t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.delete",
		Input:    mustJSON(t, map[string]any{"messageIDs": messageIDs}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "too_many_message_ids" {
		t.Fatalf("expected too_many_message_ids failure, got %+v", response)
	}
}

func TestPlatformMessageDeleteRejectsCriteriaWithoutExactIDs(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://blueclaw.test/admin/api/policy" {
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		}
		t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.delete",
		Input: mustJSON(t, map[string]any{
			"scope":      "currentChannel",
			"authoredBy": "assistant",
			"queries":    []string{"삭제대상"},
			"limit":      25,
		}),
		Context: capabilities.ToolInvokeContext{
			Platform:                "mattermost",
			ConversationID:          "channel:channel-1",
			ChannelID:               "channel-1",
			RequesterEmail:          "staff@example.com",
			RequesterPlatformUserID: "user-1",
			IsApprovalContinuation:  true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "invalid_input" {
		t.Fatalf("expected exact messageIDs to be required, got %+v", response)
	}
}

func TestPlatformMessageDeleteRejectsCriteriaWithExactIDs(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://blueclaw.test/admin/api/policy" {
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		}
		t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.delete",
		Input: mustJSON(t, map[string]any{
			"messageIDs": []string{"bot-post"},
			"queries":    []string{"삭제대상"},
		}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "invalid_input" {
		t.Fatalf("expected criteria mixed with messageIDs to be rejected, got %+v", response)
	}
}

func TestPlatformMessageSearchSkipsDeletedPosts(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/channels/channel-1/posts?page=0&per_page=100":
			return testJSONResponse(http.StatusOK, mattermostToolPostsResponse{
				Order: []string{"deleted-match", "active-match"},
				Posts: map[string]mattermostToolPost{
					"deleted-match": {ID: "deleted-match", UserID: "bot-1", ChannelID: "channel-1", Message: "삭제대상", CreateAt: 1, DeleteAt: 2},
					"active-match":  {ID: "active-match", UserID: "bot-1", ChannelID: "channel-1", Message: "삭제대상", CreateAt: 3},
				},
			}), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.search",
		Input: mustJSON(t, map[string]any{
			"scope":      "currentChannel",
			"authoredBy": "assistant",
			"queries":    []string{"삭제대상"},
		}),
		Context: capabilities.ToolInvokeContext{
			Platform:                "mattermost",
			ConversationID:          "channel:channel-1",
			ChannelID:               "channel-1",
			RequesterEmail:          "staff@example.com",
			RequesterPlatformUserID: "user-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result platformMessageSearchResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].MessageID != "active-match" {
		t.Fatalf("expected only active post candidate, got %+v", result.Candidates)
	}
}

func TestPlatformMessageSearchCurrentChannelIgnoresForeignChannelOverride(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://mattermost.test/api/v4/channels/channel-1/posts?page=0&per_page=100":
			return testJSONResponse(http.StatusOK, mattermostToolPostsResponse{
				Order: []string{"current-post"},
				Posts: map[string]mattermostToolPost{
					"current-post": {ID: "current-post", UserID: "bot-1", ChannelID: "channel-1", Message: "비밀", CreateAt: 1},
				},
			}), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		default:
			t.Fatalf("currentChannel must read only the current channel: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.search",
		Input: mustJSON(t, map[string]any{
			"scope":      "currentChannel",
			"channelID":  "circle-secret",
			"authoredBy": "assistant",
			"queries":    []string{"비밀"},
		}),
		Context: capabilities.ToolInvokeContext{
			Platform:                "mattermost",
			ConversationID:          "channel:channel-1",
			ChannelID:               "channel-1",
			RequesterEmail:          "staff@example.com",
			RequesterPlatformUserID: "user-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result platformMessageSearchResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" || len(result.Candidates) != 1 || result.Candidates[0].ChannelID != "channel-1" {
		t.Fatalf("expected currentChannel to ignore foreign override and read channel-1, got %+v", response)
	}
}

func TestPlatformMessageDeleteRejectsCriteriaWithoutMessageIDs(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://blueclaw.test/admin/api/policy" {
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		}
		t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.delete",
		Input: mustJSON(t, map[string]any{
			"scope":      "currentChannel",
			"authoredBy": "anyone",
			"queries":    []string{"삭제대상"},
		}),
		Context: capabilities.ToolInvokeContext{
			Platform:                "mattermost",
			ConversationID:          "channel:channel-1",
			ChannelID:               "channel-1",
			RequesterEmail:          "staff@example.com",
			RequesterPlatformUserID: "user-1",
			IsApprovalContinuation:  true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "invalid_input" {
		t.Fatalf("expected exact messageIDs to be required, got %+v", response)
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
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.delete",
		Input:    mustJSON(t, map[string]any{"messageIDs": []string{"user-post"}}),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:         "staff@example.com",
			IsApprovalContinuation: true,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.Outcome != capabilities.ToolOutcomeFailed || len(response.Effects) != 0 || response.ErrorCode != "post_delete_not_completed" {
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
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		}
		t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.context",
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
	if response.Status != "ok" || response.Outcome != capabilities.ToolOutcomeSucceeded || response.Effects == nil || len(response.Effects) != 0 {
		t.Fatalf("expected inspect success, got %+v", response)
	}
	var result platformMessageContextResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.RootMessageID != "root-1" || result.CurrentMessageID != "post-1" || result.BotUserID != "bot-1" {
		t.Fatalf("unexpected canonical context result: %+v", result)
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
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.search",
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
	var result platformMessageSearchResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" || len(result.Candidates) != 1 {
		t.Fatalf("expected one bot candidate, response=%+v result=%+v", response, result)
	}
	if result.AuthoredBy != "assistant" || len(result.MessageIDs) != 1 || result.MessageIDs[0] != "bot-post" {
		t.Fatalf("expected platform-facing exact delete ids, got %+v", result)
	}
}

func TestMattermostPostSearchUsesDirectMessageScope(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://blueclaw.test/admin/api/identity/resolve-recipient":
			return testJSONResponse(http.StatusOK, map[string]any{
				"status": "resolved",
				"recipient": map[string]any{
					"personID":       "person-alice",
					"displayName":    "Alice",
					"emails":         []string{"alice@example.com"},
					"externalUserID": "alice-1",
					"username":       "alice",
				},
			}), nil
		case "http://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, platformDMMattermostUser{ID: "bot-1", Username: "internkim", IsBot: true}), nil
		case "http://mattermost.test/api/v4/users/me/channels?per_page=200":
			return testJSONResponse(http.StatusOK, []mattermostToolChannel{{ID: "dm-1", Name: "alice-1__bot-1", Type: "D"}}), nil
		case "http://mattermost.test/api/v4/channels/dm-1/posts?page=0&per_page=100":
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
		ToolName: "message.search",
		Input: mustJSON(t, map[string]any{
			"scope":      "directMessage",
			"personHint": "alice@example.com",
			"authoredBy": "assistant",
		}),
		Context: capabilities.ToolInvokeContext{
			Platform:                "mattermost",
			RequesterEmail:          "alice@example.com",
			RequesterPersonID:       "person-alice",
			RequesterPlatformUserID: "alice-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result platformMessageSearchResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" || len(result.Candidates) != 1 || result.Candidates[0].ChannelID != "dm-1" {
		t.Fatalf("expected direct message candidate, response=%+v result=%+v", response, result)
	}
}

func TestMattermostPostSearchDirectMessageDeniedForNonParticipant(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://blueclaw.test/admin/api/identity/resolve-recipient":
			return testJSONResponse(http.StatusOK, map[string]any{
				"status": "resolved",
				"recipient": map[string]any{
					"personID":       "person-alice",
					"displayName":    "Alice",
					"emails":         []string{"alice@example.com"},
					"externalUserID": "alice-1",
					"username":       "alice",
				},
			}), nil
		default:
			t.Fatalf("unexpected request before authorization gate: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.search",
		Input: mustJSON(t, map[string]any{
			"scope":      "directMessage",
			"targetType": "directMessage", "personHint": "alice@example.com",
			"authoredBy": "assistant",
		}),
		Context: capabilities.ToolInvokeContext{
			Platform:                "mattermost",
			RequesterEmail:          "staff@example.com",
			RequesterPersonID:       "person-staff",
			RequesterPlatformUserID: "staff-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "dm_read_not_authorized" {
		t.Fatalf("expected non-participant DM read to be denied, got %+v", response)
	}
}

func TestMattermostPostSearchDirectMessageDeniedForAdmin(t *testing.T) {
	service := mattermostToolTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://blueclaw.test/admin/api/policy":
			return testJSONResponse(http.StatusOK, mattermostToolTestPolicy()), nil
		case "http://blueclaw.test/admin/api/identity/resolve-recipient":
			return testJSONResponse(http.StatusOK, map[string]any{
				"status": "resolved",
				"recipient": map[string]any{
					"personID":       "person-alice",
					"displayName":    "Alice",
					"emails":         []string{"alice@example.com"},
					"externalUserID": "alice-1",
					"username":       "alice",
				},
			}), nil
		default:
			t.Fatalf("admin must not reach the DM channel lookup: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message.search",
		Input: mustJSON(t, map[string]any{
			"scope":      "directMessage",
			"targetType": "directMessage", "personHint": "alice@example.com",
			"authoredBy": "assistant",
		}),
		Context: capabilities.ToolInvokeContext{
			Platform:                "mattermost",
			RequesterEmail:          "admin@example.com",
			RequesterPersonID:       "admin-1",
			RequesterPlatformUserID: "admin-1",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "dm_read_not_authorized" {
		t.Fatalf("admin must have the same access as everyone and be denied another person's DMs, got %+v", response)
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
