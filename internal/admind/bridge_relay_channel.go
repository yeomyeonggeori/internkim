package admind

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
	"gitlab.com/eastriver/internkim/internal/buzzimport"
	"gitlab.com/eastriver/internkim/internal/buzzimport/relaypublish"
)

var errBridgeChannelKindUnknown = errors.New("bridge channel kind is only known for mattermost")

type bridgeRelayChannelShape struct {
	Name        string
	RoomName    string
	Purpose     string
	ChannelType string
	Visibility  string
}

func (service *Service) ensureBridgeRelayChannel(ctx context.Context, buzzChannelID string, platform string, externalChannelID string) error {
	if !service.canWriteToBuzzRelay() {
		return nil
	}
	exists, errorValue := service.buzzChannelExists(ctx, buzzChannelID)
	if errorValue != nil {
		return errorValue
	}
	if exists {
		return nil
	}
	shape, errorValue := service.describeBridgeRelayChannel(ctx, platform, externalChannelID)
	if errorValue != nil {
		return errorValue
	}
	bootstrapSecret := buzzidentity.Secret(service.buzzKeySeed(), buzzidentity.BootstrapSubject)
	publisher, errorValue := relaypublish.Connect(ctx, service.buzzRelayEffectiveURL(), bootstrapSecret)
	if errorValue != nil {
		return errorValue
	}
	defer publisher.Close()
	errorValue = publisher.CreateChannel(ctx, bootstrapSecret, buzzChannelID, shape.Name, shape.Purpose, shape.ChannelType, shape.Visibility)
	if errorValue != nil && !strings.Contains(errorValue.Error(), "already exists") {
		return errorValue
	}
	return service.addBridgeChannelMembers(ctx, publisher, bootstrapSecret, buzzChannelID, externalChannelID)
}

// A private channel admits only its members, so the people in the Mattermost
// room have to be in the Buzz one before the mirror can publish as them.
func (service *Service) addBridgeChannelMembers(
	ctx context.Context,
	publisher *relaypublish.Publisher,
	bootstrapSecret string,
	buzzChannelID string,
	externalChannelID string,
) error {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	emails, errorValue := service.mattermostChannelMemberEmails(ctx, token, externalChannelID)
	if errorValue != nil {
		return errorValue
	}
	for email := range emails {
		secretHex := service.buzzSecretForEmail(ctx, email)
		if secretHex == "" {
			continue
		}
		pubkey, errorValue := buzzPublicKey(secretHex)
		if errorValue != nil {
			continue
		}
		if errorValue := publisher.AddMember(ctx, bootstrapSecret, buzzChannelID, pubkey); errorValue != nil {
			log.Printf("bridge channel %s: add member %s failed: %v", buzzChannelID, pubkey, errorValue)
		}
		time.Sleep(60 * time.Millisecond)
	}
	return nil
}

func (service *Service) canWriteToBuzzRelay() bool {
	return service.buzzKeySeed() != "" &&
		strings.TrimSpace(service.Configuration.BuzzRelayURL) != "" &&
		strings.TrimSpace(service.Configuration.BuzzDatabaseURL) != ""
}

func (service *Service) buzzChannelExists(ctx context.Context, buzzChannelID string) (bool, error) {
	database, errorValue := sql.Open("postgres", strings.TrimSpace(service.Configuration.BuzzDatabaseURL))
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	var exists bool
	errorValue = database.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM channels WHERE id = $1)", buzzChannelID,
	).Scan(&exists)
	return exists, errorValue
}

func (service *Service) describeBridgeRelayChannel(ctx context.Context, platform string, externalChannelID string) (bridgeRelayChannelShape, error) {
	if platform != "mattermost" {
		return bridgeRelayChannelShape{}, errBridgeChannelKindUnknown
	}
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return bridgeRelayChannelShape{}, errorValue
	}
	var channelRecord mattermostChannelRecord
	path := "/api/v4/channels/" + url.PathEscape(externalChannelID)
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &channelRecord); errorValue != nil {
		return bridgeRelayChannelShape{}, errorValue
	}
	return describeMattermostChannel(channelRecord, externalChannelID), nil
}

// Mirroring a room must never widen who can read it, so only a Mattermost
// channel anyone on the team can already join becomes an open one.
func describeMattermostChannel(channelRecord mattermostChannelRecord, externalChannelID string) bridgeRelayChannelShape {
	channelType := "stream"
	if buzzimport.IsConversationChannelType(channelRecord.Type) {
		channelType = "dm"
	}
	visibility := "private"
	if channelRecord.Type == buzzimport.OpenChannelType {
		visibility = "open"
	}
	return bridgeRelayChannelShape{
		Name:        mattermostChannelMirrorName(channelRecord, externalChannelID),
		RoomName:    strings.TrimSpace(channelRecord.Name),
		Purpose:     strings.TrimSpace(channelRecord.Purpose),
		ChannelType: channelType,
		Visibility:  visibility,
	}
}

// Mattermost leaves a conversation's display name empty and names the row after
// the user ids it joins.
func mattermostChannelMirrorName(channelRecord mattermostChannelRecord, externalChannelID string) string {
	if displayName := strings.TrimSpace(channelRecord.DisplayName); displayName != "" {
		return displayName
	}
	if name := strings.TrimSpace(channelRecord.Name); name != "" {
		return name
	}
	return externalChannelID
}
