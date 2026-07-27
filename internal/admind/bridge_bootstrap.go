package admind

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

var errBridgeSeedMissing = errors.New("buzz key seed is not configured")

func (service *Service) bridgeBuzzChannelID(externalChannelID string) (string, error) {
	seed := strings.TrimSpace(service.Configuration.BuzzKeySeed)
	if seed == "" {
		return "", errBridgeSeedMissing
	}
	return buzzidentity.ChannelID(seed, externalChannelID), nil
}

func (service *Service) resolveBridgeChannel(ctx context.Context, platform string, externalChannelID string) (string, error) {
	buzzChannelID, errorValue := service.bridgeBuzzChannelID(externalChannelID)
	if errorValue != nil {
		return "", errorValue
	}
	database, errorValue := service.openBridgeMapDatabase(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	defer database.Close()
	mapping := bridgeChannelMapping{BuzzChannelID: buzzChannelID, Platform: platform, ExternalChannelID: externalChannelID}
	if errorValue := service.recordBridgeChannel(ctx, database, mapping); errorValue != nil {
		return "", errorValue
	}
	return buzzChannelID, nil
}

func (service *Service) bootstrapBridgeChannels(ctx context.Context) (int, error) {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return 0, errorValue
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return 0, errorValue
	}
	count := 0
	for page := 0; ; page++ {
		var channels []mattermostChannelRecord
		path := "/api/v4/teams/" + url.PathEscape(teamRecord.ID) + "/channels?per_page=200&page=" + strconv.Itoa(page)
		if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &channels); errorValue != nil {
			return count, errorValue
		}
		if len(channels) == 0 {
			return count, nil
		}
		for _, channel := range channels {
			if _, errorValue := service.resolveBridgeChannel(ctx, "mattermost", channel.ID); errorValue != nil {
				return count, errorValue
			}
			count++
		}
	}
}
