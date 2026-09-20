package admind

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

const adminSeatSyncInterval = 6 * time.Hour

const buzzSeatPacing = 60 * time.Millisecond

func (service *Service) startAdminChannelSeatSync(ctx context.Context) {
	if !service.canWriteToBuzzRelay() {
		return
	}
	log.Printf("buzz admin seats: every %s, every administrator is given an owner seat in every channel", adminSeatSyncInterval)
	go func() {
		ticker := time.NewTicker(adminSeatSyncInterval)
		defer ticker.Stop()
		for {
			if unseated := seatsNotTaken(service.seatAdministratorsEverywhere(ctx, true)); unseated > 0 {
				log.Printf("buzz admin seats: %d seats the relay would not take; the rooms are named above", unseated)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

type adminSeatPlan struct {
	ChannelID string   `json:"channelID"`
	Pubkeys   []string `json:"pubkeys"`
	Unseated  int      `json:"unseated,omitempty"`
}

func adminsMissingTheirSeat(heldRoles map[string]string, adminPubkeys []string) []string {
	missing := make([]string, 0, len(adminPubkeys))
	for _, pubkey := range adminPubkeys {
		if heldRoles[pubkey] == buzzChannelOwnerRole {
			continue
		}
		missing = append(missing, pubkey)
	}
	sort.Strings(missing)
	return missing
}

const everyStreamChannelQuery = `
SELECT id::text FROM channels
WHERE community_id = ANY($1::uuid[]) AND channel_type = 'stream' AND deleted_at IS NULL
  AND name <> ALL($2::text[])
ORDER BY id`

func (service *Service) buzzCommunities(ctx context.Context, database *sql.DB) []string {
	if named := strings.TrimSpace(service.Configuration.BuzzCommunityID); named != "" {
		return []string{named}
	}
	rows, errorValue := database.QueryContext(ctx, `SELECT DISTINCT community_id::text FROM channels WHERE deleted_at IS NULL`)
	if errorValue != nil {
		log.Printf("buzz admin seats: the communities could not be read: %v", errorValue)
		return nil
	}
	defer rows.Close()
	communities := make([]string, 0)
	for rows.Next() {
		var communityID string
		if errorValue := rows.Scan(&communityID); errorValue != nil {
			return nil
		}
		communities = append(communities, communityID)
	}
	return communities
}

func everyStreamChannel(
	ctx context.Context,
	database *sql.DB,
	communities []string,
	circleRoomNames []string,
) ([]string, error) {
	if len(communities) == 0 {
		return nil, nil
	}
	rows, errorValue := database.QueryContext(
		ctx, everyStreamChannelQuery, pq.Array(communities), pq.Array(circleRoomNames))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	channelIDs := make([]string, 0)
	for rows.Next() {
		var channelID string
		if errorValue := rows.Scan(&channelID); errorValue != nil {
			return nil, errorValue
		}
		channelIDs = append(channelIDs, channelID)
	}
	return channelIDs, rows.Err()
}

func seatsNotTaken(planned []adminSeatPlan) int {
	total := 0
	for _, plan := range planned {
		total += plan.Unseated
	}
	return total
}

func seatedOwnerPubkeys(heldRoles map[string]string) []string {
	owners := make([]string, 0, len(heldRoles))
	for pubkey, role := range heldRoles {
		if isElevatedBuzzRole(role) {
			owners = append(owners, pubkey)
		}
	}
	sort.Strings(owners)
	return owners
}

func (service *Service) ownerSecretFor(ctx context.Context, heldRoles map[string]string) string {
	for _, pubkey := range seatedOwnerPubkeys(heldRoles) {
		secret, errorValue := service.buzzSigningSecretForPubkey(ctx, pubkey)
		if errorValue != nil {
			continue
		}
		return secret
	}
	return ""
}

func (service *Service) administratorPubkeys(ctx context.Context) []string {
	pubkeys := make([]string, 0)
	for email := range service.buzzAdminEmails(ctx) {
		secret, errorValue := service.personBuzzSecret(ctx, email)
		if errorValue != nil {
			log.Printf("buzz admin seats: no buzz key for an administrator: %v", errorValue)
			continue
		}
		pubkey, errorValue := buzzPublicKey(secret)
		if errorValue != nil {
			continue
		}
		pubkeys = append(pubkeys, pubkey)
	}
	sort.Strings(pubkeys)
	return pubkeys
}

func (service *Service) seatAdministratorsEverywhere(ctx context.Context, apply bool) []adminSeatPlan {
	service.adminSeating.Lock()
	defer service.adminSeating.Unlock()
	if !service.canWriteToBuzzRelay() {
		return nil
	}
	seed := service.buzzKeySeed()
	if seed == "" {
		return nil
	}
	adminPubkeys := service.administratorPubkeys(ctx)
	if len(adminPubkeys) == 0 {
		return nil
	}
	database, errorValue := sql.Open("postgres", strings.TrimSpace(service.Configuration.BuzzDatabaseURL))
	if errorValue != nil {
		log.Printf("buzz admin seats: cannot reach the relay database: %v", errorValue)
		return nil
	}
	defer database.Close()
	circleRoomNames, errorValue := service.circleRoomNames(ctx)
	if errorValue != nil {
		log.Printf("buzz admin seats: the circle rooms could not be named: %v", errorValue)
		return nil
	}
	channelIDs, errorValue := everyStreamChannel(
		ctx, database, service.buzzCommunities(ctx, database), circleRoomNames)
	if errorValue != nil {
		log.Printf("buzz admin seats: cannot read the channels: %v", errorValue)
		return nil
	}
	connections := service.newBuzzActorConnections()
	defer connections.closeAll()
	planned := make([]adminSeatPlan, 0)
	seated := make([]string, 0)
	for _, channelID := range channelIDs {
		select {
		case <-ctx.Done():
			return planned
		default:
		}
		heldRoles, errorValue := buzzChannelMemberRoles(ctx, database, channelID)
		if errorValue != nil {
			log.Printf("buzz admin seats: %s: who is in it could not be read: %v", channelID, errorValue)
			continue
		}
		missing := adminsMissingTheirSeat(heldRoles, adminPubkeys)
		if len(missing) == 0 {
			continue
		}
		plan := adminSeatPlan{ChannelID: channelID, Pubkeys: missing}
		if apply {
			plan.Unseated = service.seatAdministratorsIn(ctx, connections, channelID, heldRoles, missing, seed)
			if plan.Unseated < len(missing) {
				seated = append(seated, channelID)
			}
		}
		planned = append(planned, plan)
	}
	if len(seated) > 0 {
		if errorValue := service.tellClientsWhoIsInTheRoom(ctx, database, seated[0]); errorValue != nil {
			log.Printf("buzz admin seats: %d rooms changed and no client was told: %v", len(seated), errorValue)
		}
	}
	return planned
}

func (service *Service) seatAdministratorsIn(
	ctx context.Context,
	connections *buzzActorConnections,
	channelID string,
	heldRoles map[string]string,
	missing []string,
	seed string,
) int {
	actorSecret := service.buzzRoomActorSecret(ctx, heldRoles, seed)
	if actorSecret == buzzidentity.Secret(seed, buzzidentity.BootstrapSubject) {
		if owned := service.ownerSecretFor(ctx, heldRoles); owned != "" {
			actorSecret = owned
		}
	}
	publisher, errorValue := connections.as(ctx, actorSecret)
	if errorValue != nil {
		log.Printf("buzz admin seats: %s: the relay would not take a connection: %v", channelID, errorValue)
		return len(missing)
	}
	failed := 0
	for _, pubkey := range missing {
		errorValue := publisher.AddMember(ctx, actorSecret, channelID, pubkey, buzzChannelOwnerRole)
		if errorValue != nil && strings.Contains(errorValue.Error(), "connection closed") {
			connections.drop(actorSecret)
			reopened, reconnectError := connections.as(ctx, actorSecret)
			if reconnectError != nil {
				log.Printf("buzz admin seats: %s: the relay would not take the connection back: %v", channelID, reconnectError)
				return failed + len(missing)
			}
			publisher = reopened
			errorValue = publisher.AddMember(ctx, actorSecret, channelID, pubkey, buzzChannelOwnerRole)
		}
		if errorValue != nil {
			log.Printf("buzz admin seats: %s: %s was not seated: %v", channelID, pubkey, errorValue)
			failed++
			continue
		}
		time.Sleep(buzzSeatPacing)
	}
	return failed
}

func (service *Service) handleBuzzSeatAdmins(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebMemberRequest(request) {
		http.Error(responseWriter, "member access required", http.StatusForbidden)
		return
	}
	if !service.canWriteToBuzzRelay() {
		http.Error(responseWriter, "this device cannot write to the buzz relay", http.StatusNotImplemented)
		return
	}
	apply := request.URL.Query().Get("apply") == "true"
	planned := service.seatAdministratorsEverywhere(request.Context(), apply)
	service.writeJSON(responseWriter, map[string]any{
		"applied":  apply,
		"channels": planned,
		"unseated": seatsNotTaken(planned),
	})
}
