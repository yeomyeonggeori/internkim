package admind

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type bridgeMessageMapping struct {
	BuzzEventID       string `json:"buzzEventId"`
	Platform          string `json:"platform"`
	ExternalID        string `json:"externalId"`
	ExternalChannelID string `json:"externalChannelId"`
}

type bridgeChannelMapping struct {
	BuzzChannelID     string `json:"buzzChannelId"`
	Platform          string `json:"platform"`
	ExternalChannelID string `json:"externalChannelId"`
}

func (service *Service) openBridgeMapDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openStateDatabase(ctx, "bridge-map", ensureBridgeMapSchema, sqliteDatabaseOptions{})
}

func (service *Service) recordBridgeMessage(ctx context.Context, database *sql.DB, mapping bridgeMessageMapping) error {
	_, errorValue := database.ExecContext(ctx, `
INSERT INTO bridge_message_map (buzz_event_id, platform, external_id, external_channel_id, created_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (platform, external_id) DO NOTHING`,
		mapping.BuzzEventID, mapping.Platform, mapping.ExternalID, mapping.ExternalChannelID,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return errorValue
}

func (service *Service) bridgeMessageByExternal(ctx context.Context, database *sql.DB, platform string, externalID string) (bridgeMessageMapping, bool, error) {
	row := database.QueryRowContext(ctx, `
SELECT buzz_event_id, platform, external_id, external_channel_id
FROM bridge_message_map WHERE platform = ? AND external_id = ?`, platform, externalID)
	return scanBridgeMessage(row)
}

func (service *Service) bridgeMessageByEvent(ctx context.Context, database *sql.DB, buzzEventID string, platform string) (bridgeMessageMapping, bool, error) {
	row := database.QueryRowContext(ctx, `
SELECT buzz_event_id, platform, external_id, external_channel_id
FROM bridge_message_map WHERE buzz_event_id = ? AND platform = ?`, buzzEventID, platform)
	return scanBridgeMessage(row)
}

func scanBridgeMessage(row *sql.Row) (bridgeMessageMapping, bool, error) {
	var mapping bridgeMessageMapping
	errorValue := row.Scan(&mapping.BuzzEventID, &mapping.Platform, &mapping.ExternalID, &mapping.ExternalChannelID)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return bridgeMessageMapping{}, false, nil
	}
	if errorValue != nil {
		return bridgeMessageMapping{}, false, errorValue
	}
	return mapping, true, nil
}

func (service *Service) recordBridgeChannel(ctx context.Context, database *sql.DB, mapping bridgeChannelMapping) error {
	_, errorValue := database.ExecContext(ctx, `
INSERT INTO bridge_channel_map (buzz_channel_id, platform, external_channel_id, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT (platform, external_channel_id) DO NOTHING`,
		mapping.BuzzChannelID, mapping.Platform, mapping.ExternalChannelID,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return errorValue
}

func (service *Service) bridgeChannelByExternal(ctx context.Context, database *sql.DB, platform string, externalChannelID string) (bridgeChannelMapping, bool, error) {
	row := database.QueryRowContext(ctx, `
SELECT buzz_channel_id, platform, external_channel_id
FROM bridge_channel_map WHERE platform = ? AND external_channel_id = ?`, platform, externalChannelID)
	return scanBridgeChannel(row)
}

func (service *Service) bridgeChannelByBuzz(ctx context.Context, database *sql.DB, buzzChannelID string, platform string) (bridgeChannelMapping, bool, error) {
	row := database.QueryRowContext(ctx, `
SELECT buzz_channel_id, platform, external_channel_id
FROM bridge_channel_map WHERE buzz_channel_id = ? AND platform = ?`, buzzChannelID, platform)
	return scanBridgeChannel(row)
}

func (service *Service) listBridgeChannels(ctx context.Context, database *sql.DB, platform string) ([]bridgeChannelMapping, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT buzz_channel_id, platform, external_channel_id
FROM bridge_channel_map WHERE platform = ?`, platform)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	mappings := []bridgeChannelMapping{}
	for rows.Next() {
		var mapping bridgeChannelMapping
		if errorValue := rows.Scan(&mapping.BuzzChannelID, &mapping.Platform, &mapping.ExternalChannelID); errorValue != nil {
			return nil, errorValue
		}
		mappings = append(mappings, mapping)
	}
	return mappings, rows.Err()
}

func scanBridgeChannel(row *sql.Row) (bridgeChannelMapping, bool, error) {
	var mapping bridgeChannelMapping
	errorValue := row.Scan(&mapping.BuzzChannelID, &mapping.Platform, &mapping.ExternalChannelID)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return bridgeChannelMapping{}, false, nil
	}
	if errorValue != nil {
		return bridgeChannelMapping{}, false, errorValue
	}
	return mapping, true, nil
}
