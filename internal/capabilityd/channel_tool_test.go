package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
)

const requesterPubkeyHex = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const colleaguePubkeyHex = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func channelToolCall(toolName string, input string) capabilities.ToolInvokeRequest {
	return capabilities.ToolInvokeRequest{
		ToolName: toolName,
		Input:    json.RawMessage(input),
		Context:  capabilityprotocol.ToolInvokeContext{RequesterEmail: "member@example.com"},
	}
}

func channelService(t *testing.T, chatd *standingIn) Service {
	t.Helper()
	return serviceReaching(t, map[gateBackend]*standingIn{
		chatdOverHTTP:    chatd,
		blueclawOverHTTP: answering(memberPolicyFor("person-1", "member@example.com")),
		admindOverHTTP:   answeringPerCall(answeringTheDirectory),
	})
}

func askedOf(chatd *standingIn, capability string) []askedOfBackend {
	asked := []askedOfBackend{}
	for _, call := range chatd.asked {
		if strings.HasSuffix(call.Path, "/"+capability) {
			asked = append(asked, call)
		}
	}
	return asked
}

func TestARemovalIsAskedOfChatdAsTheRequesterAgainstTheResolvedChannel(t *testing.T) {
	chatd := answeringPerCall(answeringChannelsAsTheRequester)

	answered, errorValue := channelService(t, chatd).invokeChannelTool(context.Background(),
		channelToolCall(channelMemberDeleteToolName, `{"channelHint":"#전사-공지","personHint":"박예시"}`))

	if errorValue != nil || answered.IsError {
		t.Fatalf("expected the removal to be carried, got %+v %v", answered, errorValue)
	}
	removals := askedOf(chatd, "requester.channel.member.remove")
	if len(removals) != 1 {
		t.Fatalf("expected one removal asked of chatd, got %+v", chatd.asked)
	}
	var body map[string]string
	_ = json.Unmarshal([]byte(removals[0].Body), &body)
	if body["requesterPubkeyHex"] != requesterPubkeyHex || body["conversationID"] != "channel-1" || body["externalID"] != colleaguePubkeyHex {
		t.Fatalf("expected the requester's key, the channel's ID and the colleague's key, got %s", removals[0].Body)
	}
}

func TestAChannelChangeChatdRefusesIsAFailureCarryingItsReason(t *testing.T) {
	chatd := answeringPerCall(func(request *http.Request) (int, string) {
		if strings.HasSuffix(request.URL.Path, "/requester.channel.delete") {
			return http.StatusConflict, `{"error":"only an owner of channel channel-1 may do this","reason":"not-owner"}`
		}
		return answeringChannelsAsTheRequester(request)
	})

	answered, _ := channelService(t, chatd).invokeChannelTool(context.Background(),
		channelToolCall(channelDeleteToolName, `{"channelHint":"전사-공지"}`))

	if !answered.IsError || !strings.Contains(answered.Message, "only an owner") {
		t.Fatalf("expected chatd's refusal to reach the caller, got %+v", answered)
	}
}

func TestAChannelHintNoChannelAnswersToAsksRatherThanChanging(t *testing.T) {
	chatd := answeringPerCall(answeringChannelsAsTheRequester)

	answered, _ := channelService(t, chatd).invokeChannelTool(context.Background(),
		channelToolCall(channelLeaveToolName, `{"channelHint":"전사-공시"}`))

	if !answered.IsError || answered.ErrorCode != "interaction_required" {
		t.Fatalf("expected a near miss to become a question, got %+v", answered)
	}
	if len(askedOf(chatd, "requester.channel.leave")) != 0 {
		t.Fatal("a channel nobody named exactly must not be left")
	}
}

func TestADirectConversationIsNeverAChannel(t *testing.T) {
	chatd := answeringPerCall(answeringChannelsAsTheRequester)

	answered, _ := channelService(t, chatd).invokeChannelTool(context.Background(),
		channelToolCall(channelLeaveToolName, `{"channelHint":"dm-1"}`))

	if !answered.IsError {
		t.Fatalf("expected a direct conversation's ID to name no channel, got %+v", answered)
	}
}

func TestAChannelChangeIsApprovedAgainstTheChannelTheHintNames(t *testing.T) {
	chatd := answeringPerCall(answeringChannelsAsTheRequester)

	answered, errorValue := channelService(t, chatd).resolveChannelChangeTarget(context.Background(),
		channelToolCall(channelDeleteToolName, `{"channelHint":"전사-공지"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	target := decodeResolvedApprovalTarget(t, answered)
	if target.InputField != channelHintField || target.ID != "channel-1" || !strings.Contains(target.Title, "#전사-공지") {
		t.Fatalf("expected the approval to name the resolved channel, got %+v", target)
	}
	if len(askedOf(chatd, "requester.channel.delete")) != 0 {
		t.Fatal("resolving a target must not delete anything")
	}
}

func TestAChannelTheAgentOpensAsksAdmindToSeatTheAdministrators(t *testing.T) {
	chatd := answeringPerCall(answeringChannelsAsTheRequester)
	admindSocket := answering(`{"seated":[],"unseated":0,"unmuted":0}`)
	service := serviceReaching(t, map[gateBackend]*standingIn{
		chatdOverHTTP:       chatd,
		blueclawOverHTTP:    answering(memberPolicyFor("person-1", "member@example.com")),
		admindOverHTTP:      answeringPerCall(answeringTheDirectory),
		admindOverTheSocket: admindSocket,
	})

	answered, errorValue := service.invokeChannelTool(context.Background(),
		channelToolCall(channelAddToolName, `{"name":"launch","visibility":"open"}`))

	if errorValue != nil || answered.IsError {
		t.Fatalf("expected the channel to be opened, got %+v %v", answered, errorValue)
	}
	if len(admindSocket.asked) != 1 || admindSocket.asked[0].Path != seatAdministratorsAdmindPath || admindSocket.asked[0].Body != `{"channelID":"channel-9"}` {
		t.Fatalf("expected admind to be asked once to seat the administrators in channel-9, got %+v", admindSocket.asked)
	}
}

func TestAChannelChatdRefusesToOpenSeatsNobody(t *testing.T) {
	chatd := answeringPerCall(func(request *http.Request) (int, string) {
		if strings.HasSuffix(request.URL.Path, "/requester.channel.create") {
			return http.StatusConflict, `{"error":"a channel by that name exists"}`
		}
		return answeringChannelsAsTheRequester(request)
	})
	admindSocket := answering(`{}`)
	service := serviceReaching(t, map[gateBackend]*standingIn{
		chatdOverHTTP:       chatd,
		blueclawOverHTTP:    answering(memberPolicyFor("person-1", "member@example.com")),
		admindOverHTTP:      answeringPerCall(answeringTheDirectory),
		admindOverTheSocket: admindSocket,
	})

	answered, _ := service.invokeChannelTool(context.Background(),
		channelToolCall(channelAddToolName, `{"name":"launch","visibility":"open"}`))

	if !answered.IsError || len(admindSocket.asked) != 0 {
		t.Fatalf("a channel never opened seats nobody, got %+v and %+v", answered, admindSocket.asked)
	}
}

func TestAHandoverIsApprovedNamingThePersonTheHintResolvesTo(t *testing.T) {
	chatd := answeringPerCall(answeringChannelsAsTheRequester)

	answered, _ := channelService(t, chatd).resolveChannelChangeTarget(context.Background(),
		channelToolCall(channelOwnerUpdateToolName, `{"channelHint":"전사-공지","personHint":"person-2"}`))

	target := decodeResolvedApprovalTarget(t, answered)
	if !strings.HasSuffix(target.Title, "→ 박예시") {
		t.Fatalf("the approval names the person, not the hint, got %+v", target)
	}
}

func TestAMemberTheMessengerRefusesComesBackAsThePersonResolved(t *testing.T) {
	chatd := answeringPerCall(func(request *http.Request) (int, string) {
		if strings.HasSuffix(request.URL.Path, "/requester.channel.members.add") {
			return http.StatusOK, `{"uninvitedExternalIDs":["` + colleaguePubkeyHex + `"]}`
		}
		return answeringChannelsAsTheRequester(request)
	})

	answered, _ := channelService(t, chatd).invokeChannelTool(context.Background(),
		channelToolCall(channelMemberAddToolName, `{"channelHint":"전사-공지","personHints":["colleague@example.com"]}`))

	var result channelToolResult
	_ = json.Unmarshal(answered.Result, &result)
	if !reflect.DeepEqual(result.NotAddedPeople, []personShown{{PersonID: "person-2", Name: "박예시"}}) {
		t.Fatalf("a refused member is named by personID and name, got %s", answered.Result)
	}
}
