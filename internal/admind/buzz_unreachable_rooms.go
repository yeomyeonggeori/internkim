package admind

import (
	"context"
	"database/sql"
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
func retirePrivateRoomsNobodyIsIn(ctx context.Context, relay *sql.DB) {
	rooms, errorValue := privateRoomsNobodyIsIn(ctx, relay)
	if errorValue != nil {
		log.Printf("buzz rooms nobody is in could not be read: %v", errorValue)
		return
	}
	for _, channelID := range rooms {
		if errorValue := retireOneRoom(ctx, relay, channelID); errorValue != nil {
			log.Printf("buzz room %s, which nobody is in, could not be retired: %v", channelID, errorValue)
			return
		}
		log.Printf("buzz room %s retired: a private room nobody is left in can never be entered again", channelID)
	}
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
