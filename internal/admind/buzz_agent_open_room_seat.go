package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

const openRoomsMissingTheAgentQuery = `
SELECT c.id::text FROM channels c
WHERE c.channel_type = 'stream'
  AND c.deleted_at IS NULL
  AND c.archived_at IS NULL
  AND c.visibility = 'open'
  AND NOT EXISTS (
    SELECT 1 FROM channel_members m
    WHERE m.channel_id = c.id AND m.community_id = c.community_id
      AND m.pubkey = decode($1, 'hex') AND m.removed_at IS NULL)`

func openRoomsMissingTheAgent(ctx context.Context, relay *sql.DB, agentPubkey string) ([]string, error) {
	rows, errorValue := relay.QueryContext(ctx, openRoomsMissingTheAgentQuery, agentPubkey)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	var channelIDs []string
	for rows.Next() {
		var channelID string
		if errorValue := rows.Scan(&channelID); errorValue != nil {
			return nil, errorValue
		}
		channelIDs = append(channelIDs, channelID)
	}
	return channelIDs, rows.Err()
}

func (service *Service) seatTheAgentInOpenRooms(ctx context.Context, relay *sql.DB, connections *buzzActorConnections, seed string) error {
	agentSecret := buzzidentity.Secret(seed, buzzidentity.AgentSubject)
	agentPubkey, errorValue := buzzPublicKey(agentSecret)
	if errorValue != nil {
		return errorValue
	}
	channelIDs, errorValue := openRoomsMissingTheAgent(ctx, relay, agentPubkey)
	if errorValue != nil {
		return fmt.Errorf("open rooms without the agent could not be read: %w", errorValue)
	}
	var refusals []error
	for _, channelID := range channelIDs {
		if errorValue := connections.addMember(ctx, agentSecret, channelID, agentPubkey, ""); errorValue != nil {
			refusals = append(refusals, fmt.Errorf("seating the agent in %s: %w", channelID, errorValue))
		}
		time.Sleep(60 * time.Millisecond)
	}
	return errors.Join(refusals...)
}
