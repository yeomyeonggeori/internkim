package buzzimport

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	nostr "github.com/nbd-wtf/go-nostr"
)

type ChannelInjector struct {
	Database    *sql.DB
	CommunityID string
}

func (injector ChannelInjector) InjectMessage(ctx context.Context, channelID string, event nostr.Event) error {
	transaction, errorValue := injector.Database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	defer transaction.Rollback()

	inserted, errorValue := injector.insertEvent(ctx, transaction, channelID, event)
	if errorValue != nil {
		return errorValue
	}
	if !inserted {
		return transaction.Commit()
	}
	if errorValue := injector.insertThreadMetadata(ctx, transaction, channelID, event); errorValue != nil {
		return errorValue
	}
	if errorValue := injector.insertMentions(ctx, transaction, channelID, event); errorValue != nil {
		return errorValue
	}
	return transaction.Commit()
}

func (injector ChannelInjector) InjectProfile(ctx context.Context, event nostr.Event) error {
	eventID, errorValue := hex.DecodeString(event.ID)
	if errorValue != nil {
		return errorValue
	}
	authorPubkey, errorValue := hex.DecodeString(event.PubKey)
	if errorValue != nil {
		return errorValue
	}
	signature, errorValue := hex.DecodeString(event.Sig)
	if errorValue != nil {
		return errorValue
	}
	tagsDocument, errorValue := json.Marshal(event.Tags)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = injector.Database.ExecContext(ctx, `
		INSERT INTO events (community_id, id, pubkey, created_at, kind, tags, content, sig, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT DO NOTHING`,
		injector.CommunityID, eventID, authorPubkey, event.CreatedAt.Time().UTC(),
		event.Kind, tagsDocument, event.Content, signature,
	)
	return errorValue
}

func (injector ChannelInjector) insertEvent(ctx context.Context, transaction *sql.Tx, channelID string, event nostr.Event) (bool, error) {
	eventID, errorValue := hex.DecodeString(event.ID)
	if errorValue != nil {
		return false, errorValue
	}
	authorPubkey, errorValue := hex.DecodeString(event.PubKey)
	if errorValue != nil {
		return false, errorValue
	}
	signature, errorValue := hex.DecodeString(event.Sig)
	if errorValue != nil {
		return false, errorValue
	}
	tagsDocument, errorValue := json.Marshal(event.Tags)
	if errorValue != nil {
		return false, errorValue
	}
	result, errorValue := transaction.ExecContext(ctx, `
		INSERT INTO events (community_id, id, pubkey, created_at, kind, tags, content, sig, received_at, channel_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), $9)
		ON CONFLICT DO NOTHING`,
		injector.CommunityID, eventID, authorPubkey, event.CreatedAt.Time().UTC(),
		event.Kind, tagsDocument, event.Content, signature, channelID,
	)
	if errorValue != nil {
		return false, errorValue
	}
	affected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return false, errorValue
	}
	return affected > 0, nil
}

func (injector ChannelInjector) insertThreadMetadata(ctx context.Context, transaction *sql.Tx, channelID string, event nostr.Event) error {
	rootEventID, parentEventID := threadReferences(event)
	if rootEventID == "" && parentEventID == "" {
		return nil
	}
	eventID, errorValue := hex.DecodeString(event.ID)
	if errorValue != nil {
		return errorValue
	}
	rootBytes, errorValue := optionalEventID(rootEventID)
	if errorValue != nil {
		return errorValue
	}
	parentBytes, errorValue := optionalEventID(parentEventID)
	if errorValue != nil {
		return errorValue
	}
	createdAt := event.CreatedAt.Time().UTC()
	parentCreatedAt, errorValue := injector.eventCreatedAt(ctx, transaction, parentBytes)
	if errorValue != nil {
		return errorValue
	}
	rootCreatedAt, errorValue := injector.eventCreatedAt(ctx, transaction, rootBytes)
	if errorValue != nil {
		return errorValue
	}
	depth := 1
	if parentEventID != "" && rootEventID != "" && parentEventID != rootEventID {
		depth = 2
	}
	result, errorValue := transaction.ExecContext(ctx, `
		INSERT INTO thread_metadata
			(community_id, event_created_at, event_id, channel_id,
			 parent_event_id, parent_event_created_at,
			 root_event_id, root_event_created_at, depth, broadcast)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false)
		ON CONFLICT DO NOTHING`,
		injector.CommunityID, createdAt, eventID, channelID,
		parentBytes, parentCreatedAt, rootBytes, rootCreatedAt, depth,
	)
	if errorValue != nil {
		return errorValue
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil
	}
	if errorValue := injector.ensureThreadStub(ctx, transaction, channelID, parentBytes, parentCreatedAt); errorValue != nil {
		return errorValue
	}
	if rootEventID != parentEventID {
		if errorValue := injector.ensureThreadStub(ctx, transaction, channelID, rootBytes, rootCreatedAt); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := injector.bumpReplyCount(ctx, transaction, parentBytes, createdAt); errorValue != nil {
		return errorValue
	}
	if rootEventID != parentEventID {
		if errorValue := injector.bumpDescendantCount(ctx, transaction, rootBytes, createdAt); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (injector ChannelInjector) ensureThreadStub(ctx context.Context, transaction *sql.Tx, channelID string, eventID []byte, createdAt *time.Time) error {
	if eventID == nil || createdAt == nil {
		return nil
	}
	_, errorValue := transaction.ExecContext(ctx, `
		INSERT INTO thread_metadata
			(community_id, event_created_at, event_id, channel_id,
			 parent_event_id, parent_event_created_at,
			 root_event_id, root_event_created_at, depth, broadcast)
		VALUES ($1, $2, $3, $4, NULL, NULL, NULL, NULL, 0, false)
		ON CONFLICT DO NOTHING`,
		injector.CommunityID, *createdAt, eventID, channelID,
	)
	return errorValue
}

func (injector ChannelInjector) bumpReplyCount(ctx context.Context, transaction *sql.Tx, eventID []byte, replyAt time.Time) error {
	if eventID == nil {
		return nil
	}
	_, errorValue := transaction.ExecContext(ctx, `
		UPDATE thread_metadata
		SET reply_count = reply_count + 1,
		    descendant_count = descendant_count + 1,
		    last_reply_at = GREATEST(COALESCE(last_reply_at, $3), $3)
		WHERE community_id = $1 AND event_id = $2`,
		injector.CommunityID, eventID, replyAt,
	)
	return errorValue
}

func (injector ChannelInjector) bumpDescendantCount(ctx context.Context, transaction *sql.Tx, eventID []byte, replyAt time.Time) error {
	if eventID == nil {
		return nil
	}
	_, errorValue := transaction.ExecContext(ctx, `
		UPDATE thread_metadata
		SET descendant_count = descendant_count + 1,
		    last_reply_at = GREATEST(COALESCE(last_reply_at, $3), $3)
		WHERE community_id = $1 AND event_id = $2`,
		injector.CommunityID, eventID, replyAt,
	)
	return errorValue
}

func (injector ChannelInjector) eventCreatedAt(ctx context.Context, transaction *sql.Tx, eventID []byte) (*time.Time, error) {
	if eventID == nil {
		return nil, nil
	}
	var createdAt time.Time
	errorValue := transaction.QueryRowContext(ctx,
		`SELECT created_at FROM events WHERE community_id = $1 AND id = $2 LIMIT 1`,
		injector.CommunityID, eventID,
	).Scan(&createdAt)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return nil, nil
	}
	if errorValue != nil {
		return nil, errorValue
	}
	return &createdAt, nil
}

func (injector ChannelInjector) insertMentions(ctx context.Context, transaction *sql.Tx, channelID string, event nostr.Event) error {
	createdAt := event.CreatedAt.Time().UTC()
	eventID, errorValue := hex.DecodeString(event.ID)
	if errorValue != nil {
		return errorValue
	}
	for _, mentionPubkey := range mentionedPubkeys(event) {
		_, errorValue := transaction.ExecContext(ctx, `
			INSERT INTO event_mentions (community_id, pubkey_hex, event_id, event_created_at, channel_id, event_kind)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT DO NOTHING`,
			injector.CommunityID, mentionPubkey, eventID, createdAt, channelID, event.Kind,
		)
		if errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func threadReferences(event nostr.Event) (string, string) {
	rootEventID := ""
	parentEventID := ""
	for _, tag := range event.Tags {
		if len(tag) < 2 || tag[0] != "e" {
			continue
		}
		marker := ""
		if len(tag) > 3 {
			marker = tag[3]
		}
		switch marker {
		case "root":
			rootEventID = tag[1]
		case "reply":
			parentEventID = tag[1]
		}
	}
	if parentEventID == "" {
		parentEventID = rootEventID
	}
	if rootEventID == "" {
		rootEventID = parentEventID
	}
	return rootEventID, parentEventID
}

func mentionedPubkeys(event nostr.Event) []string {
	pubkeys := []string{}
	seen := map[string]bool{}
	for _, tag := range event.Tags {
		if len(tag) < 2 || tag[0] != "p" {
			continue
		}
		mentionPubkey := strings.ToLower(strings.TrimSpace(tag[1]))
		if len(mentionPubkey) != 64 || seen[mentionPubkey] {
			continue
		}
		seen[mentionPubkey] = true
		pubkeys = append(pubkeys, mentionPubkey)
	}
	return pubkeys
}

func optionalEventID(eventID string) ([]byte, error) {
	if strings.TrimSpace(eventID) == "" {
		return nil, nil
	}
	return hex.DecodeString(eventID)
}

func (injector ChannelInjector) InjectReaction(ctx context.Context, channelID string, event nostr.Event) error {
	eventID, errorValue := hex.DecodeString(event.ID)
	if errorValue != nil {
		return errorValue
	}
	authorPubkey, errorValue := hex.DecodeString(event.PubKey)
	if errorValue != nil {
		return errorValue
	}
	signature, errorValue := hex.DecodeString(event.Sig)
	if errorValue != nil {
		return errorValue
	}
	tagsDocument, errorValue := json.Marshal(event.Tags)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = injector.Database.ExecContext(ctx, `
		INSERT INTO events (community_id, id, pubkey, created_at, kind, tags, content, sig, received_at, channel_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), $9)
		ON CONFLICT DO NOTHING`,
		injector.CommunityID, eventID, authorPubkey, event.CreatedAt.Time().UTC(),
		event.Kind, tagsDocument, event.Content, signature, channelID,
	)
	return errorValue
}
