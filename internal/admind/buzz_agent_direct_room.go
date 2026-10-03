package admind

import (
	"context"
	"database/sql"
	"log"
	"strings"

	"github.com/lib/pq"
	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

const membersWithoutTheAgentDirectRoomQuery = `
SELECT member.pubkey FROM unnest($1::text[]) AS member(pubkey)
WHERE NOT EXISTS (
  SELECT 1 FROM channels c
  WHERE c.channel_type = 'dm'
    AND c.deleted_at IS NULL
    AND EXISTS (
      SELECT 1 FROM channel_members m
      WHERE m.channel_id = c.id AND m.community_id = c.community_id
        AND m.pubkey = decode(member.pubkey, 'hex') AND m.removed_at IS NULL)
    AND EXISTS (
      SELECT 1 FROM channel_members m
      WHERE m.channel_id = c.id AND m.community_id = c.community_id
        AND m.pubkey = decode($2, 'hex') AND m.removed_at IS NULL)
    AND (
      SELECT count(*) FROM channel_members m
      WHERE m.channel_id = c.id AND m.community_id = c.community_id
        AND m.removed_at IS NULL) = 2)`

func membersWithoutTheAgentDirectRoom(ctx context.Context, relay *sql.DB, agentPubkey string, memberPubkeys []string) ([]string, error) {
	if len(memberPubkeys) == 0 {
		return nil, nil
	}
	rows, errorValue := relay.QueryContext(ctx, membersWithoutTheAgentDirectRoomQuery, pq.Array(memberPubkeys), agentPubkey)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	var pubkeys []string
	for rows.Next() {
		var pubkey string
		if errorValue := rows.Scan(&pubkey); errorValue != nil {
			return nil, errorValue
		}
		pubkeys = append(pubkeys, pubkey)
	}
	return pubkeys, rows.Err()
}

func (service *Service) admittedMembersWithoutTheAgentDirectRoom(ctx context.Context, members []buzzMember) []buzzMember {
	seed := service.buzzKeySeed()
	if seed == "" || strings.TrimSpace(service.Configuration.ChatdEndpoint) == "" {
		return nil
	}
	agentPubkey, errorValue := buzzPublicKey(buzzidentity.Secret(seed, buzzidentity.AgentSubject))
	if errorValue != nil {
		return nil
	}
	people := make([]buzzMember, 0, len(members))
	for _, member := range members {
		if member.Email != "" && member.Pubkey != agentPubkey {
			people = append(people, member)
		}
	}
	outside := map[string]bool{}
	for _, pubkey := range service.pubkeysTheRelayMayNotHold(ctx, pubkeysOf(people)) {
		outside[pubkey] = true
	}
	admitted := make([]buzzMember, 0, len(people))
	for _, member := range people {
		if !outside[member.Pubkey] {
			admitted = append(admitted, member)
		}
	}
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		log.Printf("buzz agent direct rooms: %v", errorValue)
		return nil
	}
	missingPubkeys, errorValue := membersWithoutTheAgentDirectRoom(ctx, relay, agentPubkey, pubkeysOf(admitted))
	if errorValue != nil {
		log.Printf("buzz agent direct rooms: who has no room with the agent could not be read: %v", errorValue)
		return nil
	}
	missing := map[string]bool{}
	for _, pubkey := range missingPubkeys {
		missing[pubkey] = true
	}
	var withoutARoom []buzzMember
	for _, member := range admitted {
		if missing[member.Pubkey] {
			withoutARoom = append(withoutARoom, member)
		}
	}
	return withoutARoom
}

func (service *Service) openTheAgentDirectRoomForEveryMember(ctx context.Context, members []buzzMember) {
	opened, failed := 0, 0
	for _, member := range service.admittedMembersWithoutTheAgentDirectRoom(ctx, members) {
		if ctx.Err() != nil {
			return
		}
		if _, errorValue := service.ensureAgentDirectMessageChannel(ctx, member.Email, ""); errorValue != nil {
			log.Printf("buzz agent direct rooms: opening the room for %s failed: %v", member.Email, errorValue)
			failed++
			continue
		}
		opened++
	}
	if opened > 0 || failed > 0 {
		log.Printf("buzz agent direct rooms: opened %d, failed %d", opened, failed)
	}
}
