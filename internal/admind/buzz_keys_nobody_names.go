package admind

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"

	nostr "github.com/nbd-wtf/go-nostr"

	"github.com/lib/pq"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

func (service *Service) removeUnaccountedMemberships(
	ctx context.Context,
	database *sql.DB,
	unaccounted []string,
) (int, error) {
	if len(unaccounted) == 0 {
		return 0, nil
	}
	for _, pubkey := range unaccounted {
		if _, errorValue := hex.DecodeString(pubkey); errorValue != nil {
			return 0, errorValue
		}
	}
	unaccounted, errorValue := keysThatSaidNothing(ctx, database, unaccounted)
	if errorValue != nil || len(unaccounted) == 0 {
		return 0, errorValue
	}
	const asBytea = "ARRAY(SELECT decode(unnest($1::text[]), 'hex'))"
	rooms, errorValue := roomsHoldingMembers(ctx, database, unaccounted)
	if errorValue != nil {
		return 0, errorValue
	}
	removed, errorValue := rowsChangedBy(ctx, database,
		"DELETE FROM channel_members WHERE pubkey = ANY("+asBytea+")", pq.Array(unaccounted))
	if errorValue != nil {
		return 0, errorValue
	}
	if _, errorValue := rowsChangedBy(ctx, database,
		"DELETE FROM relay_members WHERE lower(pubkey) = ANY($1) AND role <> 'owner'",
		pq.Array(unaccounted)); errorValue != nil {
		return removed, errorValue
	}
	if _, errorValue := rowsChangedBy(ctx, database,
		"DELETE FROM events WHERE kind = 0 AND pubkey = ANY("+asBytea+")",
		pq.Array(unaccounted)); errorValue != nil {
		return removed, errorValue
	}
	for _, channelID := range rooms {
		if errorValue := service.tellClientsWhoIsInTheRoom(ctx, database, channelID); errorValue != nil {
			return removed, errorValue
		}
	}
	return removed, nil
}

func roomsHoldingMembers(ctx context.Context, database *sql.DB, pubkeys []string) ([]string, error) {
	const asBytea = "ARRAY(SELECT decode(unnest($1::text[]), 'hex'))"
	rows, errorValue := database.QueryContext(ctx,
		"SELECT DISTINCT channel_id::text FROM channel_members WHERE pubkey = ANY("+asBytea+")", pq.Array(pubkeys))
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

func (service *Service) membersNobodyAccountsFor(ctx context.Context, database *sql.DB, seed string) ([]string, error) {
	accounted, errorValue := service.accountedBuzzPubkeys(ctx, seed)
	if errorValue != nil {
		return nil, errorValue
	}
	// Looking only at the rooms is how a key nobody derives outlived every
	// sweep: a client reads relay_members to list and mention people, so a key
	// that is a member of the community and of no room is a person in the
	// messenger that no pass could see.
	rows, errorValue := database.QueryContext(ctx, `
		SELECT DISTINCT encode(pubkey, 'hex') FROM channel_members WHERE removed_at IS NULL
		UNION
		SELECT DISTINCT lower(pubkey) FROM relay_members WHERE role <> 'owner'`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	unaccounted := []string{}
	for rows.Next() {
		var pubkey string
		if errorValue := rows.Scan(&pubkey); errorValue != nil {
			return nil, errorValue
		}
		if !accounted[pubkey] {
			unaccounted = append(unaccounted, pubkey)
		}
	}
	return unaccounted, rows.Err()
}

func (service *Service) showOutWhoeverNobodyNames(ctx context.Context) error {
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		return errorValue
	}
	unaccounted, errorValue := service.membersNobodyAccountsFor(ctx, relay, service.buzzKeySeed())
	if errorValue != nil {
		return errorValue
	}
	if len(unaccounted) == 0 {
		return nil
	}
	removed, errorValue := service.removeUnaccountedMemberships(ctx, relay, unaccounted)
	if errorValue != nil {
		return fmt.Errorf("showing out a key nobody names: %w", errorValue)
	}
	if removed > 0 {
		log.Printf("buzz: showed out %d seat(s) held by keys nobody names, of %d found", removed, len(unaccounted))
	}
	return nil
}

func keysThatSaidNothing(ctx context.Context, database *sql.DB, pubkeys []string) ([]string, error) {
	rows, errorValue := database.QueryContext(ctx,
		"SELECT DISTINCT encode(pubkey, 'hex') FROM events WHERE kind = 9 AND pubkey = ANY(ARRAY(SELECT decode(unnest($1::text[]), 'hex')))",
		pq.Array(pubkeys))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	spoke := map[string]bool{}
	for rows.Next() {
		var pubkey string
		if errorValue := rows.Scan(&pubkey); errorValue != nil {
			return nil, errorValue
		}
		spoke[pubkey] = true
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	for _, pubkey := range pubkeys {
		if spoke[pubkey] {
			log.Printf("buzz: %s belongs to nobody the company names and has said something, so it stays for a person to judge", pubkey)
		}
	}
	return keysSafeToShowOut(pubkeys, spoke), nil
}

func keysSafeToShowOut(pubkeys []string, spoke map[string]bool) []string {
	silent := []string{}
	for _, pubkey := range pubkeys {
		if spoke[pubkey] {
			continue
		}
		silent = append(silent, pubkey)
	}
	return silent
}

func rowsChangedBy(ctx context.Context, database *sql.DB, statement string, argument any) (int, error) {
	result, errorValue := database.ExecContext(ctx, statement, argument)
	if errorValue != nil {
		return 0, errorValue
	}
	affected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return 0, errorValue
	}
	return int(affected), nil
}

func (service *Service) everyKeyHeldBy(ctx context.Context, seed string, email string) ([]string, error) {
	vaultSubject, errorValue := service.buzzVaultSubject(ctx, email)
	if errorValue != nil {
		return nil, errorValue
	}
	version := service.buzzIdentityVersion(vaultSubject)
	keys := []string{}
	for held := 1; held <= version; held++ {
		pubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, versionedSubject(email, held)))
		if errorValue != nil {
			continue
		}
		keys = append(keys, pubkey)
	}
	return keys, nil
}
