package admind

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/lib/pq"
	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

type buzzChannelMembershipReport struct {
	Channels  int                        `json:"channels"`
	Uninvited int                        `json:"uninvited"`
	Removed   int                        `json:"removed"`
	Rooms     []buzzChannelUninvitedRoom `json:"rooms"`
}

type buzzChannelUninvitedRoom struct {
	Channel   string   `json:"channel"`
	Uninvited []string `json:"uninvited"`
}

func (service *Service) handleBuzzChannelMembershipRepair(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "staff access required", http.StatusForbidden)
		return
	}
	report, errorValue := service.buzzMembersTheirRoomDoesNotHold(request.Context(), request.URL.Query().Get("remove") == "true")
	if errorValue != nil {
		log.Printf("buzz channel membership repair failed: %v", errorValue)
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(report)
}

func (service *Service) buzzMembersTheirRoomDoesNotHold(ctx context.Context, shouldRemove bool) (buzzChannelMembershipReport, error) {
	seed := service.buzzKeySeed()
	databaseURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	if seed == "" || databaseURL == "" {
		return buzzChannelMembershipReport{}, errors.New("buzz key seed and database url must be configured")
	}
	mappings, errorValue := service.bridgedMattermostChannels(ctx)
	if errorValue != nil {
		return buzzChannelMembershipReport{}, errorValue
	}
	relay, errorValue := sql.Open("postgres", databaseURL)
	if errorValue != nil {
		return buzzChannelMembershipReport{}, errorValue
	}
	defer relay.Close()

	report := buzzChannelMembershipReport{Rooms: []buzzChannelUninvitedRoom{}}
	for _, mapping := range mappings {
		room, errorValue := service.uninvitedInOneChannel(ctx, relay, seed, mapping)
		if errorValue != nil {
			return report, errorValue
		}
		if len(room.Uninvited) == 0 {
			continue
		}
		report.Channels++
		report.Uninvited += len(room.Uninvited)
		report.Rooms = append(report.Rooms, room)
		if !shouldRemove {
			continue
		}
		removed, errorValue := removeBuzzChannelMembers(ctx, relay, mapping.BuzzChannelID, room.Uninvited)
		if errorValue != nil {
			return report, errorValue
		}
		report.Removed += removed
		log.Printf("buzz channel members its room does not hold removed: %s %d", room.Channel, removed)
	}
	return report, nil
}

func (service *Service) uninvitedInOneChannel(
	ctx context.Context,
	relay *sql.DB,
	seed string,
	mapping bridgeChannelMapping,
) (buzzChannelUninvitedRoom, error) {
	shape, errorValue := service.describeBridgeRelayChannel(ctx, "mattermost", mapping.ExternalChannelID)
	if errorValue != nil {
		return buzzChannelUninvitedRoom{}, errorValue
	}
	if shape.Visibility == "open" || shape.ChannelType == "dm" {
		return buzzChannelUninvitedRoom{}, nil
	}
	circleID := circleIDOfRoom(shape.RoomName)
	if circleID == "" {
		return buzzChannelUninvitedRoom{}, nil
	}
	emails, errorValue := service.emailsInCircle(ctx, circleID)
	if errorValue != nil {
		return buzzChannelUninvitedRoom{}, errorValue
	}
	if len(emails) == 0 {
		return buzzChannelUninvitedRoom{}, nil
	}
	belong, errorValue := service.pubkeysOf(ctx, emails, seed)
	if errorValue != nil {
		return buzzChannelUninvitedRoom{}, errorValue
	}
	held, errorValue := buzzChannelMemberPubkeys(ctx, relay, mapping.BuzzChannelID)
	if errorValue != nil {
		return buzzChannelUninvitedRoom{}, errorValue
	}
	uninvited := []string{}
	for _, pubkey := range held {
		if !belong[pubkey] {
			uninvited = append(uninvited, pubkey)
		}
	}
	return buzzChannelUninvitedRoom{Channel: shape.Name, Uninvited: uninvited}, nil
}

func (service *Service) pubkeysOf(ctx context.Context, emails []string, seed string) (map[string]bool, error) {
	belong := map[string]bool{}
	for _, subject := range []string{buzzidentity.BootstrapSubject, buzzidentity.AgentSubject} {
		pubkey, errorValue := buzzPublicKey(buzzidentity.Secret(seed, subject))
		if errorValue != nil {
			return nil, errorValue
		}
		belong[pubkey] = true
	}
	for _, email := range emails {
		secret, errorValue := service.personBuzzSecret(ctx, email)
		if errorValue != nil {
			return nil, errorValue
		}
		pubkey, errorValue := buzzPublicKey(secret)
		if errorValue != nil {
			return nil, errorValue
		}
		belong[pubkey] = true
	}
	return belong, nil
}

func (service *Service) emailsInCircle(ctx context.Context, circleID string) ([]string, error) {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return nil, errorValue
	}
	emails := []string{}
	for email, circles := range blueclawCirclesByEmail(policyDocument) {
		for _, held := range circles {
			if strings.EqualFold(strings.TrimSpace(held), circleID) {
				emails = append(emails, email)
				break
			}
		}
	}
	return emails, nil
}

func circleIDOfRoom(roomName string) string {
	const circleRoomPrefix = "circle-"
	if !strings.HasPrefix(roomName, circleRoomPrefix) {
		return ""
	}
	return strings.TrimPrefix(roomName, circleRoomPrefix)
}

func buzzChannelMemberPubkeys(ctx context.Context, relay *sql.DB, buzzChannelID string) ([]string, error) {
	rows, errorValue := relay.QueryContext(ctx,
		"SELECT encode(pubkey, 'hex') FROM channel_members WHERE channel_id = $1 AND removed_at IS NULL", buzzChannelID)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	held := []string{}
	for rows.Next() {
		var pubkey string
		if errorValue := rows.Scan(&pubkey); errorValue != nil {
			return nil, errorValue
		}
		held = append(held, pubkey)
	}
	return held, rows.Err()
}

func removeBuzzChannelMembers(ctx context.Context, relay *sql.DB, buzzChannelID string, pubkeys []string) (int, error) {
	for _, pubkey := range pubkeys {
		if _, errorValue := hex.DecodeString(pubkey); errorValue != nil {
			return 0, errorValue
		}
	}
	result, errorValue := relay.ExecContext(ctx,
		"UPDATE channel_members SET removed_at = now() WHERE channel_id = $1 AND removed_at IS NULL"+
			" AND pubkey = ANY(ARRAY(SELECT decode(unnest($2::text[]), 'hex')))",
		buzzChannelID, pq.Array(pubkeys))
	if errorValue != nil {
		return 0, errorValue
	}
	removed, errorValue := result.RowsAffected()
	return int(removed), errorValue
}
