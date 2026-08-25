package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type buzzRetireReport struct {
	Bridged int               `json:"bridged"`
	Empty   int               `json:"empty"`
	Retired int               `json:"retired"`
	Rooms   []buzzRetiredRoom `json:"rooms"`
}

type buzzRetiredRoom struct {
	Room       string `json:"room"`
	Channel    string `json:"channel"`
	Members    int    `json:"buzzMembers"`
	Messages   int    `json:"buzzMessages"`
	IsArchived bool   `json:"roomAlreadyArchived"`
}

func (service *Service) handleBuzzChannelRetire(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "staff access required", http.StatusForbidden)
		return
	}
	report, errorValue := service.retireRoomsNobodyIsIn(request.Context(), request.URL.Query().Get("retire") == "true")
	if errorValue != nil {
		log.Printf("buzz channel retire failed: %v", errorValue)
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(report)
}

func (service *Service) retireRoomsNobodyIsIn(ctx context.Context, shouldRetire bool) (buzzRetireReport, error) {
	databaseURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	if databaseURL == "" {
		return buzzRetireReport{}, errors.New("buzz database url must be configured")
	}
	mappings, errorValue := service.bridgedMattermostChannels(ctx)
	if errorValue != nil {
		return buzzRetireReport{}, errorValue
	}
	relay, errorValue := sql.Open("postgres", databaseURL)
	if errorValue != nil {
		return buzzRetireReport{}, errorValue
	}
	defer relay.Close()

	report := buzzRetireReport{Bridged: len(mappings), Rooms: []buzzRetiredRoom{}}
	for _, mapping := range mappings {
		room, isEmpty, errorValue := service.describeEmptyRoom(ctx, relay, mapping)
		if errorValue != nil {
			return report, errorValue
		}
		if !isEmpty {
			continue
		}
		report.Empty++
		report.Rooms = append(report.Rooms, room)
		if !shouldRetire {
			continue
		}
		if errorValue := service.retireRoomAndItsMirror(ctx, relay, mapping, room.IsArchived); errorValue != nil {
			return report, errorValue
		}
		report.Retired++
		log.Printf("room nobody is in retired, and its buzz mirror with it: %s", room.Room)
	}
	return report, nil
}

func (service *Service) describeEmptyRoom(
	ctx context.Context,
	relay *sql.DB,
	mapping bridgeChannelMapping,
) (buzzRetiredRoom, bool, error) {
	shape, errorValue := service.describeBridgeRelayChannel(ctx, "mattermost", mapping.ExternalChannelID)
	if errorValue != nil {
		return buzzRetiredRoom{}, false, errorValue
	}
	if shape.Visibility == "open" || shape.ChannelType == "dm" || circleIDOfRoom(shape.RoomName) != "" {
		return buzzRetiredRoom{}, false, nil
	}
	roomMembers, errorValue := service.mattermostRoomMemberCount(ctx, mapping.ExternalChannelID)
	if errorValue != nil || roomMembers > 0 {
		return buzzRetiredRoom{}, false, errorValue
	}
	held, errorValue := buzzChannelMemberPubkeys(ctx, relay, mapping.BuzzChannelID)
	if errorValue != nil {
		return buzzRetiredRoom{}, false, errorValue
	}
	messages, errorValue := buzzChannelMessageCount(ctx, relay, mapping.BuzzChannelID)
	if errorValue != nil {
		return buzzRetiredRoom{}, false, errorValue
	}
	return buzzRetiredRoom{
		Room:       shape.RoomName,
		Channel:    shape.Name,
		Members:    len(held),
		Messages:   messages,
		IsArchived: shape.IsArchived,
	}, true, nil
}

func (service *Service) mattermostRoomMemberCount(ctx context.Context, externalChannelID string) (int, error) {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return 0, errorValue
	}
	var members []struct {
		UserID string `json:"user_id"`
	}
	path := "/api/v4/channels/" + url.PathEscape(externalChannelID) + "/members?per_page=200"
	errorValue = service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &members)
	return len(members), errorValue
}

func (service *Service) retireRoomAndItsMirror(
	ctx context.Context,
	relay *sql.DB,
	mapping bridgeChannelMapping,
	isAlreadyArchived bool,
) error {
	if !isAlreadyArchived {
		token, errorValue := service.mattermostAdminToken(ctx)
		if errorValue != nil {
			return errorValue
		}
		path := "/api/v4/channels/" + url.PathEscape(mapping.ExternalChannelID)
		if errorValue := service.mattermostRequest(ctx, http.MethodDelete, path, token, nil, nil); errorValue != nil {
			return errorValue
		}
	}
	_, errorValue := relay.ExecContext(ctx,
		"UPDATE channels SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL", mapping.BuzzChannelID)
	return errorValue
}

func buzzChannelMessageCount(ctx context.Context, relay *sql.DB, buzzChannelID string) (int, error) {
	var messages int
	errorValue := relay.QueryRowContext(ctx, `
SELECT count(*)
FROM events
WHERE kind = 9
	AND EXISTS (
		SELECT 1 FROM jsonb_array_elements(tags) AS tag
		WHERE tag->>0 = 'h' AND tag->>1 = $1
	)`, buzzChannelID).Scan(&messages)
	return messages, errorValue
}
