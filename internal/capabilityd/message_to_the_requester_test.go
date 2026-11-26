package capabilityd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

type requesterConversationRecorder struct {
	posted       []chatdMessagePostRequest
	directPosted int
}

func requesterConversationService(t *testing.T, recorder *requesterConversationRecorder) Service {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		switch {
		case isDirectoryPeopleRequest(request):
			_ = json.NewEncoder(responseWriter).Encode(map[string]any{"people": messageSendTargetPeople()})
		case request.URL.Path == "/v1/platform/buzz/message.post":
			var posted chatdMessagePostRequest
			if errorValue := json.NewDecoder(request.Body).Decode(&posted); errorValue != nil {
				t.Fatal(errorValue)
			}
			recorder.posted = append(recorder.posted, posted)
			_ = json.NewEncoder(responseWriter).Encode(chatdMessagePostResponse{MessageID: "event-pdf"})
		case request.URL.Path == "/v1/platform/buzz/dm.post":
			recorder.directPosted++
			_ = json.NewEncoder(responseWriter).Encode(map[string]string{"channelID": "dm-1", "messageID": "post-1"})
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	return Service{Configuration: Configuration{
		AdmindBaseURL:         server.URL,
		ChatdEndpoint:         server.URL,
		ChatdPlatform:         "buzz",
		BlueclawWorkspacePath: t.TempDir(),
	}}
}

func requesterDirectConversation() capabilities.ToolInvokeContext {
	return capabilities.ToolInvokeContext{
		RequesterPersonID: "person-sample",
		RequesterEmail:    "sample@example.com",
		Platform:          "buzz",
		ConversationID:    "conversation-2",
		ConversationType:  "direct",
		ChannelID:         "channel-2",
		ReplyTargetID:     "thread-2",
	}
}

func messageSendCarryingAReport(input string, toolContext capabilities.ToolInvokeContext) capabilities.ToolInvokeRequest {
	return capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    json.RawMessage(input),
		Context:  toolContext,
		Transport: capabilities.ToolInvokeTransport{
			WorkspaceFiles: []capabilities.WorkspaceFile{{
				WorkspacePath: "/workspace/private/people/person-sample/documents/보고서.pdf",
				Filename:      "보고서.pdf",
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("%PDF-1.7")),
			}},
		},
	}
}

func invokeThroughTheRoute(t *testing.T, service Service, request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
	t.Helper()
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := service.invokeCapabilityTool(context.Background(), request.ToolName, bytes.NewReader(document))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return response
}

func TestAMessageNamingTheRequesterLandsInTheirConversationWithItsFile(t *testing.T) {
	for _, personHint := range []string{"이샘플", "sample@example.com", "person-sample"} {
		t.Run(personHint, func(t *testing.T) {
			recorder := &requesterConversationRecorder{}
			service := requesterConversationService(t, recorder)
			input := `{"targetType":"directMessage","personHint":"` + personHint + `","message":"요청하신 PDF입니다","attachments":["/workspace/private/people/person-sample/documents/보고서.pdf"]}`

			response := invokeThroughTheRoute(t, service, messageSendCarryingAReport(input, requesterDirectConversation()))

			if capabilityResponseIsFailure(response) {
				t.Fatalf("a message to the requester, whom the turn gate lets through unasked, was refused: %s", response.Content)
			}
			if recorder.directPosted != 0 || len(recorder.posted) != 1 {
				t.Fatalf("expected one post in the conversation and no direct message from the requester to themselves, got posts=%+v direct=%d", recorder.posted, recorder.directPosted)
			}
			posted := recorder.posted[0]
			if posted.ThreadID != "thread-2" || len(posted.Attachments) != 1 || posted.Attachments[0].Filename != "보고서.pdf" {
				t.Fatalf("the file did not land in the conversation being answered: %+v", posted)
			}
		})
	}
}

func TestAMessageThatAlsoReachesSomeoneElseStillNeedsApproval(t *testing.T) {
	recorder := &requesterConversationRecorder{}
	service := requesterConversationService(t, recorder)
	input := `{"targetType":"directMessage","personHints":["이샘플","박예시"],"message":"공유드립니다"}`

	response := invokeThroughTheRoute(t, service, messageSendCarryingAReport(input, requesterDirectConversation()))

	if response.ErrorCode != "approval_required" {
		t.Fatalf("a send that reaches a colleague ran without approval: %+v", response)
	}
	if len(recorder.posted) != 0 || recorder.directPosted != 0 {
		t.Fatalf("nothing may be sent before approval, got posts=%+v direct=%d", recorder.posted, recorder.directPosted)
	}
}

func TestAMessageNamingTheRequesterFromAChannelSaysWhereToReply(t *testing.T) {
	recorder := &requesterConversationRecorder{}
	service := requesterConversationService(t, recorder)
	channelThread := requesterDirectConversation()
	channelThread.ConversationType = "channel"
	channelThread.ChannelID = "channel-1"
	channelThread.ReplyTargetID = "thread-1"
	input := `{"targetType":"directMessage","personHint":"이샘플","message":"요청하신 PDF입니다","attachments":["/workspace/private/people/person-sample/documents/보고서.pdf"]}`

	response := invokeThroughTheRoute(t, service, messageSendCarryingAReport(input, channelThread))

	if response.ErrorCode != "unsupported_target" || !strings.Contains(response.Content, "targetType=currentThread") {
		t.Fatalf("expected the steer to reply in the thread, got %+v", response)
	}
	if len(recorder.posted) != 0 || recorder.directPosted != 0 {
		t.Fatalf("nothing may be sent, got posts=%+v direct=%d", recorder.posted, recorder.directPosted)
	}
}
