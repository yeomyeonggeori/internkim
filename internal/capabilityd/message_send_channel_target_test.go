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

type channelStandIn struct {
	channels      []chatdChannel
	canList       bool
	postedChannel []string
}

func messageSendChannelTestChannels() []chatdChannel {
	return []chatdChannel{
		{ChannelID: "channel-general", Name: "general"},
		{ChannelID: "channel-dev", Name: "dev"},
		{ChannelID: "channel-devops", Name: "devops"},
		{ChannelID: "channel-ops-a", Name: "ops"},
		{ChannelID: "channel-ops-b", Name: "#ops"},
		{ChannelID: "room-1", Name: "lobby"},
	}
}

func channelStandInService(t *testing.T, standIn *channelStandIn) Service {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1/platform/buzz/channels.list":
			if !standIn.canList {
				http.Error(responseWriter, "no channel directory", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(responseWriter).Encode(map[string]any{"channels": standIn.channels})
		case "/v1/platform/buzz/message.post":
			var posted chatdMessagePostRequest
			_ = json.NewDecoder(request.Body).Decode(&posted)
			standIn.postedChannel = append(standIn.postedChannel, posted.ChannelID)
			_ = json.NewEncoder(responseWriter).Encode(map[string]string{"messageID": "post-1", "channelID": posted.ChannelID})
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	return Service{Configuration: Configuration{ChatdEndpoint: server.URL, ChatdPlatform: "buzz"}}
}

func channelPostService(t *testing.T) (Service, *channelStandIn) {
	t.Helper()
	standIn := &channelStandIn{channels: messageSendChannelTestChannels(), canList: true}
	return channelStandInService(t, standIn), standIn
}

func channelTargetInput(fields map[string]string) json.RawMessage {
	document := map[string]string{"targetType": "channel", "message": "공지"}
	for key, value := range fields {
		document[key] = value
	}
	encoded, _ := json.Marshal(document)
	return encoded
}

func TestAUniqueChannelNameResolvesToTheExactChannel(t *testing.T) {
	service, _ := channelPostService(t)

	for _, channelName := range []string{"general", "#General", " general "} {
		response := resolveApprovalTargetThroughRoute(t, service, "message_send", channelTargetInput(map[string]string{"channelName": channelName}))

		target := decodeResolvedApprovalTarget(t, response)
		if target.InputField != "channelID" || target.ID != "channel-general" || !strings.Contains(target.Title, "#general") || !strings.Contains(target.Title, "channel-general") {
			t.Fatalf("the approval names exactly which channel receives it for %q, got %+v", channelName, target)
		}
	}
}

func TestAChannelNameThatPrefixesAnotherStillResolvesToItself(t *testing.T) {
	service, _ := channelPostService(t)

	target := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", channelTargetInput(map[string]string{"channelName": "dev"})))

	if target.ID != "channel-dev" {
		t.Fatalf("a channel name is exact, so dev is not devops, got %+v", target)
	}
}

func TestAnExactChannelIDResolvesToItselfWhateverNameAccompaniesIt(t *testing.T) {
	service, _ := channelPostService(t)
	byName := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", channelTargetInput(map[string]string{"channelName": "general"})))

	byIdentity := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, "message_send", channelTargetInput(map[string]string{"channelID": byName.ID})))

	if !reflect.DeepEqual(byIdentity, byName) {
		t.Fatalf("narrowing the hold to the identity is only safe while it resolves to itself, got %+v then %+v", byName, byIdentity)
	}
}

func TestAChannelNameSeveralChannelsAnswerToAsksWhichByID(t *testing.T) {
	service, _ := channelPostService(t)

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", channelTargetInput(map[string]string{"channelName": "ops"}))

	assertPlatformDMStructuredFailure(t, response, "error", "interaction_required", "target_resolution", true, true)
	for _, channelID := range []string{"channel-ops-a", "channel-ops-b"} {
		if !strings.Contains(response.Content, channelID) {
			t.Fatalf("the question names each candidate by its ID, got %q", response.Content)
		}
	}
}

func TestAChannelNameNothingMatchesOffersTheNearestAndNoneOfThese(t *testing.T) {
	service, _ := channelPostService(t)

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", channelTargetInput(map[string]string{"channelName": "genral"}))

	assertPlatformDMStructuredFailure(t, response, "error", "interaction_required", "target_resolution", true, true)
	if !strings.Contains(response.Content, "channel-general") || !strings.Contains(response.Content, "none of these") {
		t.Fatalf("a near miss is a question with an exit, got %q", response.Content)
	}
}

func TestAFragmentOfAChannelNameOffersTheChannelAsAQuestion(t *testing.T) {
	service, _ := channelPostService(t)

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", channelTargetInput(map[string]string{"channelName": "gen"}))

	assertPlatformDMStructuredFailure(t, response, "error", "interaction_required", "target_resolution", true, true)
	if !strings.Contains(response.Content, "channel-general") {
		t.Fatalf("a fragment is a guess to confirm, never a channel to post to, got %q", response.Content)
	}
}

func TestAChannelNameNothingIsNearIsNotFound(t *testing.T) {
	service, _ := channelPostService(t)

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", channelTargetInput(map[string]string{"channelName": "marketing"}))

	assertPlatformDMStructuredFailure(t, response, "error", "channel_not_found", "channel_resolve", false, false)
}

func TestAChannelIDOneCharacterOffIsNeverApproximated(t *testing.T) {
	service, _ := channelPostService(t)

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", channelTargetInput(map[string]string{"channelID": "lobby1"}))

	assertPlatformDMStructuredFailure(t, response, "error", "channel_not_found", "channel_resolve", false, false)
	if strings.Contains(response.Content, "room-1") {
		t.Fatalf("an invented identifier is not offered a neighbour, got %q", response.Content)
	}
}

func TestAMessengerWithoutAChannelDirectoryLeavesTheCallToItsOwnResolution(t *testing.T) {
	standIn := &channelStandIn{channels: messageSendChannelTestChannels()}
	service := channelStandInService(t, standIn)

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", channelTargetInput(map[string]string{"channelName": "general"}))

	if response.Status != "no_target" {
		t.Fatalf("a device whose messenger cannot list channels keeps posting as it did, got %+v", response)
	}
}

func TestAReplyInTheCurrentChannelNeedsNoChannelLookup(t *testing.T) {
	service, _ := channelPostService(t)

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", json.RawMessage(`{"targetType":"currentChannel","message":"네"}`))

	if response.Status != "no_target" {
		t.Fatalf("expected no target, got %+v", response)
	}
}

func TestExecutionPostsToTheChannelTheHoldResolved(t *testing.T) {
	service, standIn := channelPostService(t)
	approvedInput := channelTargetInput(map[string]string{"channelName": "#General", "channelID": "channel-general"})

	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    approvedInput,
		Context:  capabilities.ToolInvokeContext{IsApprovalContinuation: true},
	})

	if errorValue != nil || response.Status != "sent" {
		t.Fatalf("expected the narrowed post to send, got %+v %v", response, errorValue)
	}
	if len(standIn.postedChannel) != 1 || standIn.postedChannel[0] != "channel-general" {
		t.Fatalf("the post went to a channel other than the approved one: %v", standIn.postedChannel)
	}
}
