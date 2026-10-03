package admind

import (
	"context"
	"database/sql"
	"encoding/hex"
	"sort"

	"github.com/lib/pq"
)

func buzzChannelMemberPubkeys(ctx context.Context, relay *sql.DB, buzzChannelID string) ([]string, error) {
	roles, errorValue := buzzChannelMemberRoles(ctx, relay, buzzChannelID)
	if errorValue != nil {
		return nil, errorValue
	}
	held := make([]string, 0, len(roles))
	for pubkey := range roles {
		held = append(held, pubkey)
	}
	sort.Strings(held)
	return held, nil
}

func buzzChannelMemberRoles(ctx context.Context, relay *sql.DB, buzzChannelID string) (map[string]string, error) {
	rows, errorValue := relay.QueryContext(ctx,
		"SELECT encode(pubkey, 'hex'), role::text FROM channel_members WHERE channel_id = $1 AND removed_at IS NULL", buzzChannelID)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	held := map[string]string{}
	for rows.Next() {
		var pubkey, role string
		if errorValue := rows.Scan(&pubkey, &role); errorValue != nil {
			return nil, errorValue
		}
		held[pubkey] = role
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
