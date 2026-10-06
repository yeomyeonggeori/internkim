package capabilityd

import (
	"context"
	"strings"
)

type chatdChannel struct {
	ChannelID string `json:"channelID"`
	Name      string `json:"name"`
}

func (channel chatdChannel) hintIdentifiers() []string { return []string{channel.ChannelID} }

func (channel chatdChannel) hintMatchesTitle(hint string) bool {
	return canonicalChannelHint(hint) != "" && canonicalChannelHint(hint) == canonicalChannelHint(channel.Name)
}

func (channel chatdChannel) hintNearness(hint string) float64 {
	canonicalHint := canonicalChannelHint(hint)
	canonicalName := canonicalChannelHint(channel.Name)
	nearness := typoNearness(canonicalHint, canonicalName)
	if canonicalHint != "" && canonicalName != "" && strings.Contains(canonicalName, canonicalHint) && nearness == 0 {
		return 0.5
	}
	return nearness
}

func canonicalChannelHint(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(value), "#")))
}

func (service Service) chatdChannels(ctx context.Context) ([]chatdChannel, error) {
	var response struct {
		Channels []chatdChannel `json:"channels"`
	}
	if errorValue := service.chatdRequest(ctx, "channels.list", struct{}{}, &response); errorValue != nil {
		return nil, errorValue
	}
	return response.Channels, nil
}

func channelTitle(channel chatdChannel) string {
	return "#" + canonicalChannelHint(channel.Name) + " (" + strings.TrimSpace(channel.ChannelID) + ")"
}
