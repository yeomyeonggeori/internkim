package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func answeringManyChannelsAsTheRequester(request *http.Request) (int, string) {
	switch {
	case strings.HasSuffix(request.URL.Path, "/requester.conversations.list"):
		return http.StatusOK, `{"conversations":[{"id":"channel-c","name":"charlie","kind":"group"},{"id":"dm-1","name":"박예시","kind":"dm"},{"id":"channel-a","name":"alpha","kind":"group"},{"id":"channel-e","name":"echo","kind":"group"}]}`
	case strings.HasSuffix(request.URL.Path, "/requester.channels.open.list"):
		return http.StatusOK, `{"channels":[{"id":"channel-d","name":"delta","kind":"group"},{"id":"channel-b","name":"bravo","kind":"group"},{"id":"channel-a","name":"alpha","kind":"group"}]}`
	}
	return answeringChannelsAsTheRequester(request)
}

func listChannels(t *testing.T, service Service, input string) channelListResult {
	t.Helper()
	answered, errorValue := service.invokeChannelListTool(context.Background(), channelToolCall(channelListToolName, input))
	if errorValue != nil || answered.IsError {
		t.Fatalf("expected channels, got %+v %v", answered, errorValue)
	}
	var result channelListResult
	if errorValue := json.Unmarshal(answered.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	return result
}

func TestChannelPagesReadInNameOrderHoldEveryChannelOnce(t *testing.T) {
	service := channelService(t, answeringPerCall(answeringManyChannelsAsTheRequester))

	names := []string{}
	cursor := ""
	for page := 0; page < 5; page++ {
		input := `{"limit":2}`
		if cursor != "" {
			input = `{"limit":2,"cursor":"` + cursor + `"}`
		}
		result := listChannels(t, service, input)
		if result.TotalCount != 5 {
			t.Fatalf("expected five channels in all on every page, got %d", result.TotalCount)
		}
		for _, channel := range result.Channels {
			names = append(names, channel.Name)
		}
		cursor = result.NextCursor
		if cursor == "" {
			break
		}
	}

	if strings.Join(names, ",") != "alpha,bravo,charlie,delta,echo" {
		t.Fatalf("expected every channel once in name order, got %v", names)
	}
}

func TestAListedChannelSaysWhetherTheRequesterIsInIt(t *testing.T) {
	result := listChannels(t, channelService(t, answeringPerCall(answeringManyChannelsAsTheRequester)), `{}`)

	isMemberByID := map[string]bool{}
	for _, channel := range result.Channels {
		isMemberByID[channel.ChannelID] = channel.IsMember
	}
	if !isMemberByID["channel-a"] || !isMemberByID["channel-c"] || isMemberByID["channel-b"] || isMemberByID["channel-d"] {
		t.Fatalf("expected joined channels marked and open ones not, got %+v", result.Channels)
	}
	if result.NextCursor != "" {
		t.Fatalf("expected no cursor when every channel fits, got %q", result.NextCursor)
	}
}

func TestChannelsAreListedAsTheRequesterSeesThem(t *testing.T) {
	chatd := answeringPerCall(answeringManyChannelsAsTheRequester)

	result := listChannels(t, channelService(t, chatd), `{}`)

	for _, capability := range []string{"requester.conversations.list", "requester.channels.open.list"} {
		asked := askedOf(chatd, capability)
		if len(asked) != 1 || !strings.Contains(asked[0].Body, requesterPubkeyHex) {
			t.Fatalf("expected %s asked once as the requester, got %+v", capability, chatd.asked)
		}
	}
	if len(askedOf(chatd, "channels.list")) != 0 {
		t.Fatal("the agent's own channel list must not answer for the requester")
	}
	for _, channel := range result.Channels {
		if channel.ChannelID == "dm-1" {
			t.Fatal("a direct conversation is not a channel")
		}
	}
}

func TestAChannelCursorNoResultGaveIsRefused(t *testing.T) {
	chatd := answeringPerCall(answeringManyChannelsAsTheRequester)

	answered, _ := channelService(t, chatd).invokeChannelListTool(context.Background(),
		channelToolCall(channelListToolName, `{"cursor":"not-a-cursor"}`))

	if !answered.IsError || answered.ErrorCode != "invalid_input" || !strings.Contains(answered.Message, "cursor") {
		t.Fatalf("expected the cursor refused as input, got %+v", answered)
	}
	if len(chatd.asked) != 0 {
		t.Fatalf("expected nothing asked of chatd for a refused input, got %+v", chatd.asked)
	}
}

func TestAChannelPageLargerThanAHundredIsRefused(t *testing.T) {
	answered, _ := channelService(t, answeringPerCall(answeringManyChannelsAsTheRequester)).invokeChannelListTool(context.Background(),
		channelToolCall(channelListToolName, `{"limit":101}`))

	if !answered.IsError || answered.ErrorCode != "invalid_input" {
		t.Fatalf("expected the limit refused as input, got %+v", answered)
	}
}
