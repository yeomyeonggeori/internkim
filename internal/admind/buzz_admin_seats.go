package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

const buzzSeatPacing = 60 * time.Millisecond

const seatableChannelQuery = `
SELECT EXISTS (SELECT 1 FROM channels c
WHERE c.id = $1::uuid AND c.channel_type = 'stream'
  AND c.deleted_at IS NULL AND c.archived_at IS NULL
  AND NOT (c.visibility = 'private' AND c.created_by = decode($2, 'hex')))`

const everSeatedQuery = `SELECT encode(pubkey, 'hex') FROM channel_members WHERE channel_id = $1::uuid`

type seatableAdministrator struct {
	Pubkey   string
	MemberID string
}

type administratorSeating struct {
	Seated   []string `json:"seated"`
	Unseated int      `json:"unseated"`
	Unmuted  int      `json:"unmuted"`
}

func channelTakesAdministrators(ctx context.Context, database *sql.DB, channelID string, agentPubkey string) (bool, error) {
	var isSeatable bool
	errorValue := database.QueryRowContext(ctx, seatableChannelQuery, channelID, agentPubkey).Scan(&isSeatable)
	return isSeatable, errorValue
}

func everSeatedPubkeys(ctx context.Context, database *sql.DB, channelID string) (map[string]bool, error) {
	rows, errorValue := database.QueryContext(ctx, everSeatedQuery, channelID)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	seated := map[string]bool{}
	for rows.Next() {
		var pubkey string
		if errorValue := rows.Scan(&pubkey); errorValue != nil {
			return nil, errorValue
		}
		seated[pubkey] = true
	}
	return seated, rows.Err()
}

type administratorSeatPlan struct {
	Newcomers []seatableAdministrator
	Raised    []seatableAdministrator
}

func planAdministratorSeats(heldRoles map[string]string, everSeated map[string]bool, administrators []seatableAdministrator) administratorSeatPlan {
	plan := administratorSeatPlan{Newcomers: []seatableAdministrator{}, Raised: []seatableAdministrator{}}
	for _, administrator := range administrators {
		role, isSeated := heldRoles[administrator.Pubkey]
		switch {
		case !everSeated[administrator.Pubkey]:
			plan.Newcomers = append(plan.Newcomers, administrator)
		case isSeated && !isElevatedBuzzRole(role):
			plan.Raised = append(plan.Raised, administrator)
		}
	}
	byPubkey := func(administrators []seatableAdministrator) func(int, int) bool {
		return func(left, right int) bool { return administrators[left].Pubkey < administrators[right].Pubkey }
	}
	sort.Slice(plan.Newcomers, byPubkey(plan.Newcomers))
	sort.Slice(plan.Raised, byPubkey(plan.Raised))
	return plan
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

func (service *Service) seatableAdministrators(ctx context.Context) ([]seatableAdministrator, error) {
	members, errorValue := service.companyMembers(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	administrators := make([]seatableAdministrator, 0)
	for _, member := range members {
		if member.HasLeftTheCompany() || normalizeAdminUserRole(member.Role) != adminUserRoleAdmin {
			continue
		}
		secret, errorValue := service.personBuzzSecret(ctx, normalizedRosterEmail(member.Email))
		if errorValue != nil {
			log.Printf("buzz admin seats: no buzz key for an administrator: %v", errorValue)
			continue
		}
		pubkey, errorValue := buzzPublicKey(secret)
		if errorValue != nil {
			continue
		}
		administrators = append(administrators, seatableAdministrator{Pubkey: pubkey, MemberID: strings.TrimSpace(member.MemberID)})
	}
	return administrators, nil
}

func (service *Service) seatAdministratorsInNewChannel(ctx context.Context, channelID string, requesterEmail string) (administratorSeating, int, string) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return administratorSeating{}, http.StatusServiceUnavailable, "this device holds no messenger key seed"
	}
	database, errorValue := service.buzzDatabase()
	if errorValue != nil {
		return administratorSeating{}, http.StatusServiceUnavailable, "the relay database cannot be reached: " + errorValue.Error()
	}
	agentPubkey, errorValue := buzzPublicKey(buzzidentity.Secret(seed, buzzidentity.AgentSubject))
	if errorValue != nil {
		return administratorSeating{}, http.StatusServiceUnavailable, "the agent has no messenger key: " + errorValue.Error()
	}
	isSeatable, errorValue := channelTakesAdministrators(ctx, database, channelID, agentPubkey)
	if errorValue != nil {
		return administratorSeating{}, http.StatusBadRequest, "the channel could not be read: " + errorValue.Error()
	}
	if !isSeatable {
		return administratorSeating{Seated: []string{}}, http.StatusOK, ""
	}
	heldRoles, errorValue := buzzChannelMemberRoles(ctx, database, channelID)
	if errorValue != nil {
		return administratorSeating{}, http.StatusServiceUnavailable, "who is in the channel could not be read: " + errorValue.Error()
	}
	if !service.ownsChannel(ctx, heldRoles, requesterEmail) {
		return administratorSeating{}, http.StatusForbidden, "only an owner of the channel may seat its administrators"
	}
	administrators, errorValue := service.seatableAdministrators(ctx)
	if errorValue != nil {
		return administratorSeating{}, http.StatusServiceUnavailable, "the company's administrators could not be read: " + errorValue.Error()
	}
	everSeated, errorValue := everSeatedPubkeys(ctx, database, channelID)
	if errorValue != nil {
		return administratorSeating{}, http.StatusServiceUnavailable, "who has been in the channel could not be read: " + errorValue.Error()
	}
	plan := planAdministratorSeats(heldRoles, everSeated, administrators)
	missing := append(append([]seatableAdministrator{}, plan.Newcomers...), plan.Raised...)
	seating := administratorSeating{Seated: []string{}}
	if len(missing) == 0 {
		return seating, http.StatusOK, ""
	}
	isNewcomer := map[string]bool{}
	for _, administrator := range plan.Newcomers {
		isNewcomer[administrator.Pubkey] = true
	}
	connections := service.newBuzzActorConnections()
	defer connections.closeAll()
	seated := service.seatAdministratorsIn(ctx, connections, channelID, heldRoles, missing, seed)
	seating.Unseated = len(missing) - len(seated)
	for _, administrator := range seated {
		seating.Seated = append(seating.Seated, administrator.Pubkey)
		if !isNewcomer[administrator.Pubkey] {
			continue
		}
		if errorValue := service.centralPlane().MuteConversation(ctx, administrator.MemberID, channelID); errorValue != nil {
			log.Printf("buzz admin seats: %s: a seated administrator was not muted: %v", channelID, errorValue)
			seating.Unmuted++
		}
	}
	if len(seated) > 0 {
		if errorValue := service.tellClientsWhoIsInTheRoom(ctx, database, channelID); errorValue != nil {
			log.Printf("buzz admin seats: %s changed and no client was told: %v", channelID, errorValue)
		}
	}
	return seating, http.StatusOK, ""
}

func (service *Service) ownsChannel(ctx context.Context, heldRoles map[string]string, requesterEmail string) bool {
	secret, errorValue := service.personBuzzSecret(ctx, requesterEmail)
	if errorValue != nil {
		return false
	}
	pubkey, errorValue := buzzPublicKey(secret)
	return errorValue == nil && isElevatedBuzzRole(heldRoles[pubkey])
}

func (service *Service) seatAdministratorsIn(
	ctx context.Context,
	connections *buzzActorConnections,
	channelID string,
	heldRoles map[string]string,
	missing []seatableAdministrator,
	seed string,
) []seatableAdministrator {
	actorSecret := service.buzzRoomActorSecret(ctx, heldRoles, seed)
	if actorSecret == buzzidentity.Secret(seed, buzzidentity.BootstrapSubject) {
		if owned := service.ownerSecretFor(ctx, heldRoles); owned != "" {
			actorSecret = owned
		}
	}
	publisher, errorValue := connections.as(ctx, actorSecret)
	if errorValue != nil {
		log.Printf("buzz admin seats: %s: the relay would not take a connection: %v", channelID, errorValue)
		return nil
	}
	seated := make([]seatableAdministrator, 0, len(missing))
	for _, administrator := range missing {
		errorValue := publisher.AddMember(ctx, actorSecret, channelID, administrator.Pubkey, buzzChannelOwnerRole)
		if errorValue != nil && strings.Contains(errorValue.Error(), "connection closed") {
			connections.drop(actorSecret)
			reopened, reconnectError := connections.as(ctx, actorSecret)
			if reconnectError != nil {
				log.Printf("buzz admin seats: %s: the relay would not take the connection back: %v", channelID, reconnectError)
				return seated
			}
			publisher = reopened
			errorValue = publisher.AddMember(ctx, actorSecret, channelID, administrator.Pubkey, buzzChannelOwnerRole)
		}
		if errorValue != nil {
			log.Printf("buzz admin seats: %s: %s was not seated: %v", channelID, administrator.Pubkey, errorValue)
			continue
		}
		seated = append(seated, administrator)
		time.Sleep(buzzSeatPacing)
	}
	return seated
}

func (service *Service) handleBuzzSeatAdmins(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(responseWriter, "seating administrators is a POST naming the new channel", http.StatusMethodNotAllowed)
		return
	}
	requesterEmail := service.actorEmailAllowingAssertedRequester(request)
	if requesterEmail == "" {
		http.Error(responseWriter, "seating administrators names the channel owner who asks", http.StatusForbidden)
		return
	}
	if !service.canWriteToBuzzRelay() {
		http.Error(responseWriter, "this device cannot write to the buzz relay", http.StatusNotImplemented)
		return
	}
	var asked struct {
		ChannelID string `json:"channelID"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&asked); errorValue != nil || strings.TrimSpace(asked.ChannelID) == "" {
		http.Error(responseWriter, "seating administrators names the channel that was just opened as channelID", http.StatusBadRequest)
		return
	}
	seating, status, failure := service.seatAdministratorsInNewChannel(request.Context(), strings.TrimSpace(asked.ChannelID), requesterEmail)
	if failure != "" {
		http.Error(responseWriter, failure, status)
		return
	}
	service.writeJSON(responseWriter, seating)
}
