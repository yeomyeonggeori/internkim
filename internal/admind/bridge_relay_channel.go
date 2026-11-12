package admind

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/mattermostadmin"
)

const (
	mattermostOpenChannelType   = "O"
	mattermostDirectChannelType = "D"
	mattermostGroupChannelType  = "G"
)

var errBridgeChannelKindUnknown = errors.New("bridge channel kind is only known for mattermost")

type bridgeRelayChannelShape struct {
	Name        string
	RoomName    string
	Purpose     string
	ChannelType string
	Visibility  string
	IsArchived  bool
}

func (service *Service) canWriteToBuzzRelay() bool {
	return service.buzzKeySeed() != "" &&
		strings.TrimSpace(service.Configuration.BuzzRelayURL) != "" &&
		strings.TrimSpace(service.Configuration.BuzzDatabaseURL) != ""
}

func (service *Service) describeBridgeRelayChannel(ctx context.Context, platform string, externalChannelID string) (bridgeRelayChannelShape, error) {
	if platform != "mattermost" {
		return bridgeRelayChannelShape{}, errBridgeChannelKindUnknown
	}
	token, errorValue := service.mattermostAdmin().AdminToken(ctx)
	if errorValue != nil {
		return bridgeRelayChannelShape{}, errorValue
	}
	var channelRecord mattermostadmin.ChannelRecord
	path := "/api/v4/channels/" + url.PathEscape(externalChannelID)
	if errorValue := service.mattermostAdmin().Request(ctx, http.MethodGet, path, token, nil, &channelRecord); errorValue != nil {
		return bridgeRelayChannelShape{}, errorValue
	}
	return describeMattermostChannel(channelRecord, externalChannelID), nil
}

// Mirroring a room must never widen who can read it, so only a Mattermost
// channel anyone on the team can already join becomes an open one.
func describeMattermostChannel(channelRecord mattermostadmin.ChannelRecord, externalChannelID string) bridgeRelayChannelShape {
	channelType := "stream"
	if channelRecord.Type == mattermostDirectChannelType || channelRecord.Type == mattermostGroupChannelType {
		channelType = "dm"
	}
	visibility := "private"
	if channelRecord.Type == mattermostOpenChannelType {
		visibility = "open"
	}
	return bridgeRelayChannelShape{
		Name:        mattermostChannelMirrorName(channelRecord, externalChannelID),
		RoomName:    strings.TrimSpace(channelRecord.Name),
		IsArchived:  channelRecord.DeleteAt != 0,
		Purpose:     strings.TrimSpace(channelRecord.Purpose),
		ChannelType: channelType,
		Visibility:  visibility,
	}
}

// Mattermost leaves a conversation's display name empty and names the row after
// the user ids it joins.
func mattermostChannelMirrorName(channelRecord mattermostadmin.ChannelRecord, externalChannelID string) string {
	if displayName := strings.TrimSpace(channelRecord.DisplayName); displayName != "" {
		return displayName
	}
	if name := strings.TrimSpace(channelRecord.Name); name != "" {
		return name
	}
	return externalChannelID
}
