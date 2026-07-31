package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"gitlab.com/eastriver/internkim/internal/buzzimport"
)

const orphanRootMarker = "이전 대화"

// repairChannelOrphans re-links replies that an incremental import stranded under
// a synthetic "이전 대화" root back onto their real imported root, then removes the
// now-empty synthetic root. It never wipes and never touches membership, so it
// cannot cause an access outage. Matching is deterministic: an event maps to a
// Mattermost post by (author pubkey, created_at second).
func repairChannelOrphans(ctx context.Context, database *sql.DB, communityID, buzzChannelID, bootstrapPubkey string, posts []buzzimport.MattermostPost, authorPubkeys map[string]string, apply bool) (relinked int, deletedRoots int) {
	mmPostByID := map[string]buzzimport.MattermostPost{}
	for _, post := range posts {
		mmPostByID[post.ID] = post
	}
	buzzIDByKey := map[string]string{}
	keyByBuzzID := map[string]string{}
	rows, errorValue := database.QueryContext(ctx, `SELECT encode(id,'hex'), encode(pubkey,'hex'), extract(epoch from created_at)::bigint FROM events WHERE community_id=$1 AND channel_id=$2 AND kind=9`, communityID, buzzChannelID)
	if errorValue != nil {
		log.Printf("repair: query events failed for %s: %v", buzzChannelID, errorValue)
		return 0, 0
	}
	for rows.Next() {
		var idHex, pubHex string
		var second int64
		if errorValue := rows.Scan(&idHex, &pubHex, &second); errorValue != nil {
			continue
		}
		key := fmt.Sprintf("%s:%d", pubHex, second)
		buzzIDByKey[key] = idHex
		keyByBuzzID[idHex] = key
	}
	rows.Close()

	mmIDByKey := map[string]string{}
	for _, post := range posts {
		pubkey := authorPubkeys[post.UserID]
		if pubkey == "" {
			continue
		}
		mmIDByKey[fmt.Sprintf("%s:%d", pubkey, post.CreatedAt.Unix())] = post.ID
	}

	realRootByBuzzID := func(replyBuzzID string) string {
		mmID := mmIDByKey[keyByBuzzID[replyBuzzID]]
		if mmID == "" {
			return ""
		}
		rootPost, isKnown := mmPostByID[mmPostByID[mmID].RootID]
		if !isKnown {
			return ""
		}
		return buzzIDByKey[fmt.Sprintf("%s:%d", authorPubkeys[rootPost.UserID], rootPost.CreatedAt.Unix())]
	}

	synthRoots := []string{}
	synthRows, errorValue := database.QueryContext(ctx, `SELECT encode(id,'hex') FROM events WHERE community_id=$1 AND channel_id=$2 AND content=$3 AND pubkey=decode($4,'hex')`, communityID, buzzChannelID, orphanRootMarker, bootstrapPubkey)
	if errorValue != nil {
		log.Printf("repair: query synthetic roots failed for %s: %v", buzzChannelID, errorValue)
		return 0, 0
	}
	for synthRows.Next() {
		var idHex string
		if errorValue := synthRows.Scan(&idHex); errorValue == nil {
			synthRoots = append(synthRoots, idHex)
		}
	}
	synthRows.Close()

	for _, synthRoot := range synthRoots {
		replies := queryReplyEventIDs(ctx, database, communityID, synthRoot)
		if len(replies) == 0 {
			if apply {
				deleteEvent(ctx, database, communityID, synthRoot)
			}
			deletedRoots++
			continue
		}
		realRoot := ""
		for _, reply := range replies {
			if candidate := realRootByBuzzID(reply); candidate != "" && candidate != synthRoot {
				realRoot = candidate
				break
			}
		}
		if realRoot == "" {
			log.Printf("repair: %s synthetic root %s: %d replies, no real root matched (genuine orphan, kept)", buzzChannelID, synthRoot[:12], len(replies))
			continue
		}
		log.Printf("repair: %s synthetic root %s -> real root %s, re-linking %d replies%s", buzzChannelID, synthRoot[:12], realRoot[:12], len(replies), applySuffix(apply))
		if apply {
			relinkReplies(ctx, database, communityID, synthRoot, realRoot, replies)
			deleteEvent(ctx, database, communityID, synthRoot)
			recomputeDescendantCount(ctx, database, communityID, realRoot)
		}
		relinked += len(replies)
		deletedRoots++
	}
	return relinked, deletedRoots
}

func queryReplyEventIDs(ctx context.Context, database *sql.DB, communityID, rootHex string) []string {
	rows, errorValue := database.QueryContext(ctx, `SELECT encode(event_id,'hex') FROM thread_metadata WHERE community_id=$1 AND root_event_id=decode($2,'hex')`, communityID, rootHex)
	if errorValue != nil {
		return nil
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var idHex string
		if errorValue := rows.Scan(&idHex); errorValue == nil {
			ids = append(ids, idHex)
		}
	}
	return ids
}

func relinkReplies(ctx context.Context, database *sql.DB, communityID, synthRoot, realRoot string, replies []string) {
	for _, reply := range replies {
		if _, errorValue := database.ExecContext(ctx, `UPDATE events SET tags = replace(tags::text, $1, $2)::jsonb WHERE community_id=$3 AND id=decode($4,'hex')`, `"`+synthRoot+`"`, `"`+realRoot+`"`, communityID, reply); errorValue != nil {
			log.Printf("repair: relink tags %s failed: %v", reply[:12], errorValue)
		}
		if _, errorValue := database.ExecContext(ctx, `UPDATE thread_metadata SET root_event_id=decode($1,'hex'), parent_event_id=decode($1,'hex') WHERE community_id=$2 AND event_id=decode($3,'hex')`, realRoot, communityID, reply); errorValue != nil {
			log.Printf("repair: relink metadata %s failed: %v", reply[:12], errorValue)
		}
	}
}

func deleteEvent(ctx context.Context, database *sql.DB, communityID, eventHex string) {
	database.ExecContext(ctx, `DELETE FROM event_mentions WHERE community_id=$1 AND event_id=decode($2,'hex')`, communityID, eventHex)
	database.ExecContext(ctx, `DELETE FROM thread_metadata WHERE community_id=$1 AND event_id=decode($2,'hex')`, communityID, eventHex)
	database.ExecContext(ctx, `DELETE FROM events WHERE community_id=$1 AND id=decode($2,'hex')`, communityID, eventHex)
}

func recomputeDescendantCount(ctx context.Context, database *sql.DB, communityID, rootHex string) {
	database.ExecContext(ctx, `UPDATE thread_metadata SET descendant_count=(SELECT count(*) FROM thread_metadata t WHERE t.community_id=$1 AND t.root_event_id=decode($2,'hex')), reply_count=(SELECT count(*) FROM thread_metadata t WHERE t.community_id=$1 AND t.parent_event_id=decode($2,'hex')) WHERE community_id=$1 AND event_id=decode($2,'hex')`, communityID, rootHex)
}

func applySuffix(apply bool) string {
	if apply {
		return " [APPLIED]"
	}
	return " [dry-run]"
}
