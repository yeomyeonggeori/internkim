package admind

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

// The relay takes a new member into a private room only from someone already in
// it, so a private room with nobody left can never be entered again, whoever
// emptied it: a sweep taking back a key, or its last person leaving. An open
// room stays, since anyone may still join it.
const privateRoomsNobodyIsInQuery = `
SELECT c.id::text FROM channels c
WHERE c.visibility = 'private' AND c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM channel_members m WHERE m.channel_id = c.id AND m.removed_at IS NULL)
ORDER BY c.id`

// Retiring hides the room from every listing and keeps what was said in it.
func retirePrivateRoomsNobodyIsIn(ctx context.Context, relay *sql.DB) error {
	rooms, errorValue := privateRoomsNobodyIsIn(ctx, relay)
	if errorValue != nil {
		return fmt.Errorf("rooms nobody is in could not be read: %w", errorValue)
	}
	for _, channelID := range rooms {
		if errorValue := retireOneRoom(ctx, relay, channelID); errorValue != nil {
			return fmt.Errorf("room %s, which nobody is in, could not be retired: %w", channelID, errorValue)
		}
		log.Printf("buzz room %s retired: a private room nobody is left in can never be entered again", channelID)
	}
	return nil
}

func privateRoomsNobodyIsIn(ctx context.Context, relay *sql.DB) ([]string, error) {
	rows, errorValue := relay.QueryContext(ctx, privateRoomsNobodyIsInQuery)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	rooms := []string{}
	for rows.Next() {
		var channelID string
		if errorValue := rows.Scan(&channelID); errorValue != nil {
			return nil, errorValue
		}
		rooms = append(rooms, channelID)
	}
	return rooms, rows.Err()
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
