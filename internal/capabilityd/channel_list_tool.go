package capabilityd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const channelListToolName = "channel_list"

const (
	channelListDefaultLimit = 50
	channelListMaximumLimit = 100
)

type channelListInput struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

type channelListPosition struct {
	Name      string `json:"name"`
	ChannelID string `json:"channelID"`
}

type listedChannel struct {
	ChannelID string `json:"channelID"`
	Name      string `json:"name"`
	IsMember  bool   `json:"isMember"`
}

type channelListResult struct {
	Channels   []listedChannel `json:"channels"`
	TotalCount int             `json:"totalCount"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

func (service Service) invokeChannelListTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
		return response, nil
	}
	input, after, errorValue := decodeChannelListInput(request.Input)
	if errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	requesterPubkeyHex, response, isRefused := service.requesterMessengerKey(ctx, request)
	if isRefused {
		return response, nil
	}
	channels, errorValue := service.channelsTheRequesterSees(ctx, requesterPubkeyHex)
	if errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolFailureForError("channel_list", "platform_unavailable", errorValue)), nil
	}
	return platformToolSuccessResponse(request.ToolName, "ok", pageOfChannels(channels, after, input.Limit))
}

func (service Service) channelsTheRequesterSees(ctx context.Context, requesterPubkeyHex string) ([]listedChannel, error) {
	var conversations struct {
		Conversations []requesterConversation `json:"conversations"`
	}
	if errorValue := service.askChatdAsTheRequester(ctx, "requester.conversations.list", requesterPubkeyHex, map[string]any{}, &conversations); errorValue != nil {
		return nil, errorValue
	}
	var open struct {
		Channels []requesterConversation `json:"channels"`
	}
	if errorValue := service.askChatdAsTheRequester(ctx, "requester.channels.open.list", requesterPubkeyHex, map[string]any{}, &open); errorValue != nil {
		return nil, errorValue
	}
	return mergedChannels(groupChannels(conversations.Conversations), groupChannels(open.Channels)), nil
}

func mergedChannels(joined []chatdChannel, open []chatdChannel) []listedChannel {
	channels := []listedChannel{}
	isListed := map[string]bool{}
	for _, channel := range joined {
		channels = append(channels, listedChannel{ChannelID: channel.ChannelID, Name: channel.Name, IsMember: true})
		isListed[channel.ChannelID] = true
	}
	for _, channel := range open {
		if isListed[channel.ChannelID] {
			continue
		}
		channels = append(channels, listedChannel{ChannelID: channel.ChannelID, Name: channel.Name})
		isListed[channel.ChannelID] = true
	}
	sort.Slice(channels, func(left, right int) bool {
		return channelListPositionOf(channels[left]).isBefore(channelListPositionOf(channels[right]))
	})
	return channels
}

func pageOfChannels(channels []listedChannel, after *channelListPosition, limit int) channelListResult {
	remaining := channels
	if after != nil {
		start := sort.Search(len(channels), func(index int) bool {
			return after.isBefore(channelListPositionOf(channels[index]))
		})
		remaining = channels[start:]
	}
	result := channelListResult{Channels: remaining, TotalCount: len(channels)}
	if len(remaining) > limit {
		result.Channels = remaining[:limit]
		result.NextCursor = encodeChannelListCursor(channelListPositionOf(remaining[limit-1]))
	}
	return result
}

func channelListPositionOf(channel listedChannel) channelListPosition {
	return channelListPosition{Name: channel.Name, ChannelID: channel.ChannelID}
}

func (position channelListPosition) isBefore(other channelListPosition) bool {
	if position.Name != other.Name {
		return position.Name < other.Name
	}
	return position.ChannelID < other.ChannelID
}

func encodeChannelListCursor(position channelListPosition) string {
	document, _ := json.Marshal(position)
	return base64.RawURLEncoding.EncodeToString(document)
}

func decodeChannelListCursor(cursor string) (*channelListPosition, error) {
	malformed := errors.New("cursor is not one a channel_list result gave; omit it to start from the first page")
	document, errorValue := base64.RawURLEncoding.DecodeString(cursor)
	if errorValue != nil {
		return nil, malformed
	}
	var position channelListPosition
	if json.Unmarshal(document, &position) != nil || position.ChannelID == "" {
		return nil, malformed
	}
	return &position, nil
}

func decodeChannelListInput(document json.RawMessage) (channelListInput, *channelListPosition, error) {
	input := channelListInput{}
	if len(bytes.TrimSpace(document)) > 0 {
		if errorValue := json.Unmarshal(document, &input); errorValue != nil {
			return channelListInput{}, nil, errorValue
		}
	}
	if input.Limit < 0 || input.Limit > channelListMaximumLimit {
		return channelListInput{}, nil, errors.New("limit must be between 1 and 100")
	}
	if input.Limit == 0 {
		input.Limit = channelListDefaultLimit
	}
	input.Cursor = strings.TrimSpace(input.Cursor)
	if input.Cursor == "" {
		return input, nil, nil
	}
	after, errorValue := decodeChannelListCursor(input.Cursor)
	return input, after, errorValue
}
