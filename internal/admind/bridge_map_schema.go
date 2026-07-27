package admind

import (
	"context"
	"database/sql"
)

func ensureBridgeMapSchema(ctx context.Context, database *sql.DB) error {
	if _, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS bridge_message_map (
	buzz_event_id TEXT NOT NULL,
	platform TEXT NOT NULL,
	external_id TEXT NOT NULL,
	external_channel_id TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	PRIMARY KEY (platform, external_id)
)`); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE UNIQUE INDEX IF NOT EXISTS bridge_message_map_event_platform
	ON bridge_message_map (buzz_event_id, platform)`); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS bridge_channel_map (
	buzz_channel_id TEXT NOT NULL,
	platform TEXT NOT NULL,
	external_channel_id TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (platform, external_channel_id)
)`); errorValue != nil {
		return errorValue
	}
	_, errorValue := database.ExecContext(ctx, `
CREATE UNIQUE INDEX IF NOT EXISTS bridge_channel_map_buzz_platform
	ON bridge_channel_map (buzz_channel_id, platform)`)
	return errorValue
}
