package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	nostr "github.com/nbd-wtf/go-nostr"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

type buzzGhostRoom struct {
	ChannelID            string   `json:"channelID"`
	Members              []string `json:"members"`
	MetadataParticipants []string `json:"metadataParticipants"`
	Messages             int      `json:"messages"`
	Reason               string   `json:"reason"`
}

type buzzGhostRoomsReport struct {
	Scanned  int             `json:"scanned"`
	Ghosts   []buzzGhostRoom `json:"ghosts"`
	HeldBack []buzzGhostRoom `json:"heldBack"`
	Retired  int             `json:"retired"`
}

// A direct room the import's bookkeeping key sits in, a room only the agent's
// own keys remain in, and a room with at most one member and nothing ever said
// are not conversations anybody is having. Retiring one hides it from every
// listing; the messages it may hold stay in the store.
func (service *Service) handleBuzzGhostRooms(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "staff access required", http.StatusForbidden)
		return
	}
	report, errorValue := service.retireGhostRooms(request.Context(), request.URL.Query().Get("apply") == "true")
	if errorValue != nil {
		log.Printf("buzz ghost rooms failed: %v", errorValue)
		http.Error(responseWriter, "buzz_ghost_rooms_failed: "+errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, report)
}

func (service *Service) retireGhostRooms(ctx context.Context, apply bool) (buzzGhostRoomsReport, error) {
	seed := service.buzzKeySeed()
	databaseURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	if seed == "" || databaseURL == "" {
		return buzzGhostRoomsReport{}, errors.New("buzz key seed and database url must be configured")
	}
	keys, errorValue := buzzServiceKeys(ctx, service, seed)
	if errorValue != nil {
		return buzzGhostRoomsReport{}, errorValue
	}

	database, errorValue := sql.Open("postgres", databaseURL)
	if errorValue != nil {
		return buzzGhostRoomsReport{}, errorValue
	}
	defer database.Close()

	rooms, errorValue := readDirectRooms(ctx, database)
	if errorValue != nil {
		return buzzGhostRoomsReport{}, errorValue
	}

	report := buzzGhostRoomsReport{Ghosts: []buzzGhostRoom{}, HeldBack: []buzzGhostRoom{}}
	report.Scanned = len(rooms)
	for _, room := range rooms {
		classified, isGhost, isHeldBack := classifyDirectRoom(room, keys)
		if isGhost {
			report.Ghosts = append(report.Ghosts, classified)
		} else if isHeldBack {
			report.HeldBack = append(report.HeldBack, classified)
		}
	}
	if !apply {
		return report, nil
	}
	for _, ghost := range report.Ghosts {
		if errorValue := retireOneRoom(ctx, database, ghost.ChannelID); errorValue != nil {
			return report, errorValue
		}
		report.Retired++
		log.Printf("ghost direct room retired: %s (%s)", ghost.ChannelID, ghost.Reason)
	}
	return report, nil
}

// The bootstrap key made the channels during the import, and the bot's old
// email-derived key is a leftover of the same era; a room either of them sits
// in is the import's own bookkeeping. The agent key is different: a person's
// direct room with the agent is a real conversation, so it only marks a ghost
// when nobody else is in the room.
type buzzServiceKeySet struct {
	bookkeeping map[string]bool
	agent       map[string]bool
	derivable   map[string]bool
}

func buzzServiceKeys(ctx context.Context, service *Service, seed string) (buzzServiceKeySet, error) {
	keys := buzzServiceKeySet{bookkeeping: map[string]bool{}, agent: map[string]bool{}, derivable: map[string]bool{}}
	bootstrapPubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
	if errorValue != nil {
		return keys, errorValue
	}
	keys.bookkeeping[bootstrapPubkey] = true
	agentPubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, buzzidentity.AgentSubject))
	if errorValue != nil {
		return keys, errorValue
	}
	keys.agent[agentPubkey] = true
	if botEmail := service.mattermostBotBuzzEmail(ctx); botEmail != "" {
		pubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, botEmail))
		if errorValue == nil {
			keys.bookkeeping[pubkey] = true
		}
	}
	emails, errorValue := service.companyPeopleEmails(ctx)
	if errorValue != nil {
		return keys, errorValue
	}
	for _, email := range emails {
		version := service.buzzIdentityVersion(service.buzzVaultSubject(ctx, email))
		for held := 1; held <= version; held++ {
			pubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, versionedSubject(email, held)))
			if errorValue != nil {
				return keys, errorValue
			}
			keys.derivable[pubkey] = true
		}
	}
	for pubkey := range keys.bookkeeping {
		keys.derivable[pubkey] = true
	}
	for pubkey := range keys.agent {
		keys.derivable[pubkey] = true
	}
	return keys, nil
}

type buzzDirectRoom struct {
	channelID            string
	members              []string
	metadataParticipants []string
	messages             int
}

func readDirectRooms(ctx context.Context, database *sql.DB) ([]buzzDirectRoom, error) {
	rows, errorValue := database.QueryContext(ctx, `
		SELECT c.id::text,
			coalesce(array_agg(encode(m.pubkey,'hex')) FILTER (WHERE m.removed_at IS NULL), '{}'),
			coalesce((SELECT count(*) FROM events e WHERE e.channel_id::text = c.id::text AND e.kind = 9), 0)
		FROM channels c LEFT JOIN channel_members m ON m.channel_id = c.id
		WHERE c.channel_type = 'dm' AND c.deleted_at IS NULL
		GROUP BY c.id`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	rooms := []buzzDirectRoom{}
	byID := map[string]int{}
	for rows.Next() {
		var room buzzDirectRoom
		var members []byte
		if errorValue := rows.Scan(&room.channelID, &members, &room.messages); errorValue != nil {
			return nil, errorValue
		}
		room.members = parsePostgresTextArray(string(members))
		byID[room.channelID] = len(rooms)
		rooms = append(rooms, room)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}

	tagRows, errorValue := database.QueryContext(ctx,
		"SELECT channel_id::text, tags FROM events WHERE kind IN (39000, 39002) AND channel_id IS NOT NULL")
	if errorValue != nil {
		return nil, errorValue
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var channelID string
		var tagsDocument []byte
		if errorValue := tagRows.Scan(&channelID, &tagsDocument); errorValue != nil {
			return nil, errorValue
		}
		index, isKnown := byID[channelID]
		if !isKnown {
			continue
		}
		var tags [][]string
		if errorValue := json.Unmarshal(tagsDocument, &tags); errorValue != nil {
			continue
		}
		for _, tag := range tags {
			if len(tag) >= 2 && tag[0] == "p" && !contains(rooms[index].metadataParticipants, tag[1]) {
				rooms[index].metadataParticipants = append(rooms[index].metadataParticipants, tag[1])
			}
		}
	}
	return rooms, tagRows.Err()
}

func classifyDirectRoom(room buzzDirectRoom, keys buzzServiceKeySet) (buzzGhostRoom, bool, bool) {
	classified := buzzGhostRoom{
		ChannelID:            room.channelID,
		Members:              room.members,
		MetadataParticipants: room.metadataParticipants,
		Messages:             room.messages,
	}
	holdsBookkeepingKey := false
	holdsOnlyServiceKeys := len(room.members) > 0
	for _, member := range room.members {
		if keys.bookkeeping[member] {
			holdsBookkeepingKey = true
		}
		if !keys.bookkeeping[member] && !keys.agent[member] {
			holdsOnlyServiceKeys = false
		}
	}
	switch {
	case holdsBookkeepingKey:
		classified.Reason = "the import's bookkeeping key sits in the room"
		return classified, true, false
	case holdsOnlyServiceKeys:
		classified.Reason = "only the service's own keys remain"
		return classified, true, false
	case len(room.members) <= 1 && room.messages == 0:
		classified.Reason = "at most one member and nothing said"
		return classified, true, false
	case len(room.members) <= 1 && counterpartNoLongerExists(room, keys):
		classified.Reason = "the counterpart's key no longer belongs to anybody"
		return classified, true, false
	case len(room.members) <= 1:
		classified.Reason = "one member but messages exist"
		return classified, false, true
	}
	return classified, false, false
}

func retireOneRoom(ctx context.Context, database *sql.DB, channelID string) error {
	if _, errorValue := database.ExecContext(ctx,
		"DELETE FROM events WHERE kind IN (39000,39001,39002) AND channel_id = $1", channelID); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx,
		"UPDATE channel_members SET removed_at = now() WHERE channel_id = $1 AND removed_at IS NULL", channelID); errorValue != nil {
		return errorValue
	}
	_, errorValue := database.ExecContext(ctx,
		"UPDATE channels SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL", channelID)
	return errorValue
}

func parsePostgresTextArray(value string) []string {
	trimmed := strings.Trim(value, "{}")
	if trimmed == "" {
		return []string{}
	}
	parts := strings.Split(trimmed, ",")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		cleaned = append(cleaned, strings.Trim(strings.TrimSpace(part), `"`))
	}
	return cleaned
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// The room's metadata names who it was opened between. When every named
// counterpart is a key that is not a member and that no identity this device
// can derive owns, the other side of the conversation does not exist any more.
func counterpartNoLongerExists(room buzzDirectRoom, keys buzzServiceKeySet) bool {
	counterparts := 0
	for _, participant := range room.metadataParticipants {
		if contains(room.members, participant) {
			continue
		}
		counterparts++
		if keys.derivable[participant] {
			return false
		}
	}
	return counterparts > 0
}
