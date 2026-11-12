package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
	"github.com/yeomyeonggeori/internkim/internal/relaypublish"
)

func pubkeysOf(members []buzzMember) []string {
	pubkeys := make([]string, 0, len(members))
	for _, member := range members {
		pubkeys = append(pubkeys, member.Pubkey)
	}
	return pubkeys
}

// The roster decides who still has to be let in, rather than whoever happened
// to be new to this pass. A grant that failed used to be made once and never
// again, and asking costs one query when everyone is already in.
func (service *Service) letOntoTheRelay(ctx context.Context, pubkeys []string) error {
	var refusals []error
	for _, pubkey := range service.pubkeysTheRelayMayNotHold(ctx, pubkeys) {
		refusals = append(refusals, service.grantRelayMembership(ctx, pubkey))
	}
	return errors.Join(refusals...)
}

// A roster that cannot be read answers everyone: the grant is idempotent, and a
// pass that skipped a person because it could not look is the silence this loop
// exists to end.
func (service *Service) pubkeysTheRelayMayNotHold(ctx context.Context, pubkeys []string) []string {
	if len(pubkeys) == 0 {
		return nil
	}
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		return pubkeys
	}
	lowered := make([]string, 0, len(pubkeys))
	for _, pubkey := range pubkeys {
		lowered = append(lowered, strings.ToLower(pubkey))
	}
	rows, errorValue := relay.QueryContext(ctx,
		"SELECT lower(pubkey) FROM relay_members WHERE lower(pubkey) = ANY($1)", pq.Array(lowered))
	if errorValue != nil {
		return pubkeys
	}
	defer rows.Close()
	held := map[string]bool{}
	for rows.Next() {
		var pubkey string
		if errorValue := rows.Scan(&pubkey); errorValue != nil {
			return pubkeys
		}
		held[pubkey] = true
	}
	if errorValue := rows.Err(); errorValue != nil {
		return pubkeys
	}
	var missing []string
	for _, pubkey := range pubkeys {
		if !held[strings.ToLower(pubkey)] {
			missing = append(missing, pubkey)
		}
	}
	return missing
}

func (service *Service) grantRelayMembership(ctx context.Context, pubkey string) error {
	execution := service.buzzAdminCommand(ctx, "add-member", "--pubkey", pubkey)
	if execution == nil {
		return nil
	}
	output, errorValue := execution.CombinedOutput()
	if errorValue != nil && !strings.Contains(string(output), "already") {
		return fmt.Errorf("letting %s onto the relay: %w (%s)", pubkey, errorValue, strings.TrimSpace(string(output)))
	}
	return nil
}

// buzz-admin as this host runs it: the binary, the relay's database and its
// public address from the admind configuration, and the relay's signing key when
// the host holds one. Nil when the host names no binary or database.
func (service *Service) buzzAdminCommand(ctx context.Context, arguments ...string) *exec.Cmd {
	command := strings.TrimSpace(service.Configuration.BuzzAdminCommandPath)
	databaseURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	if command == "" || databaseURL == "" {
		return nil
	}
	execution := exec.CommandContext(ctx, command, arguments...)
	execution.Env = append(os.Environ(), "DATABASE_URL="+databaseURL, "RELAY_URL="+service.buzzRelayEffectiveURL())
	if relayKey := service.buzzRelayKeySetting(); relayKey != "" {
		execution.Env = append(execution.Env, relayKey)
	}
	return execution
}

func (service *Service) buzzRelayKeySetting() string {
	content, errorValue := os.ReadFile(strings.TrimSpace(service.Configuration.BuzzRelayKeyPath))
	if errorValue != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		if setting := strings.TrimSpace(line); strings.HasPrefix(setting, "BUZZ_RELAY_PRIVATE_KEY=") && setting != "BUZZ_RELAY_PRIVATE_KEY=" {
			return setting
		}
	}
	return ""
}

const (
	relayConnectAttempts   = 10
	relayConnectRetryDelay = 3 * time.Second
)

func (service *Service) connectToTheRelayOnceItAnswers(ctx context.Context, actorSecretHex string) (*relaypublish.Publisher, error) {
	var refusal error
	for attempt := 0; attempt < relayConnectAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(relayConnectRetryDelay):
			}
		}
		publisher, errorValue := relaypublish.Connect(ctx, service.buzzRelayEffectiveURL(), service.Configuration.BuzzRelayURL, actorSecretHex)
		if errorValue == nil {
			return publisher, nil
		}
		refusal = errorValue
	}
	return nil, refusal
}

// An admin in a room administers it, so the room gives them the role that lets
// them add and remove people. A room holds as many owners as it has admins in
// it; the relay refuses only to leave one with none.
const buzzChannelOwnerRole = "owner"

func (service *Service) buzzAdminEmails(ctx context.Context) map[string]bool {
	admins := map[string]bool{}
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return admins
	}
	for _, record := range records {
		if normalizeAdminUserRole(record.Role) != adminUserRoleAdmin {
			continue
		}
		admins[strings.ToLower(strings.TrimSpace(record.Email))] = true
	}
	return admins
}

func buzzChannelRoleFor(adminEmails map[string]bool, email string) string {
	if adminEmails[strings.ToLower(strings.TrimSpace(email))] {
		return buzzChannelOwnerRole
	}
	return ""
}

// One membership pass signs as a different person in each room, and a relay
// connection authenticates one key, so the pass holds one connection per key
// it has spoken as.
type buzzActorConnections struct {
	service *Service
	open    map[string]*relaypublish.Publisher
}

func (service *Service) newBuzzActorConnections() *buzzActorConnections {
	return &buzzActorConnections{service: service, open: map[string]*relaypublish.Publisher{}}
}

func (connections *buzzActorConnections) as(ctx context.Context, actorSecretHex string) (*relaypublish.Publisher, error) {
	if known, isOpen := connections.open[actorSecretHex]; isOpen {
		return known, nil
	}
	publisher, errorValue := connections.service.connectToTheRelayOnceItAnswers(ctx, actorSecretHex)
	if errorValue != nil {
		return nil, errorValue
	}
	connections.open[actorSecretHex] = publisher
	return publisher, nil
}

func (connections *buzzActorConnections) addMember(ctx context.Context, actorSecretHex string, channelID string, pubkey string, role string) error {
	publisher, errorValue := connections.as(ctx, actorSecretHex)
	if errorValue != nil {
		return fmt.Errorf("connecting to the relay: %w", errorValue)
	}
	errorValue = publisher.AddMember(ctx, actorSecretHex, channelID, pubkey, role)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "connection closed") {
		return errorValue
	}
	connections.drop(actorSecretHex)
	publisher, errorValue = connections.as(ctx, actorSecretHex)
	if errorValue != nil {
		return fmt.Errorf("the relay would not take the connection back: %w", errorValue)
	}
	return publisher.AddMember(ctx, actorSecretHex, channelID, pubkey, role)
}

func (connections *buzzActorConnections) drop(actorSecretHex string) {
	if known, isOpen := connections.open[actorSecretHex]; isOpen {
		known.Close()
		delete(connections.open, actorSecretHex)
	}
}

func (connections *buzzActorConnections) closeAll() {
	for _, publisher := range connections.open {
		publisher.Close()
	}
	connections.open = map[string]*relaypublish.Publisher{}
}

func (service *Service) buzzRoomActorSecret(
	ctx context.Context,
	heldRoles map[string]string,
	seed string,
) string {
	adminSecretsByPubkey := map[string]string{}
	for email := range service.buzzAdminEmails(ctx) {
		secret, errorValue := service.personBuzzSecret(ctx, email)
		if errorValue != nil {
			continue
		}
		pubkey, errorValue := buzzPublicKey(secret)
		if errorValue != nil {
			continue
		}
		adminSecretsByPubkey[pubkey] = secret
	}
	agentSecret := buzzidentity.Secret(seed, buzzidentity.AgentSubject)
	agentPubkey, errorValue := buzzPublicKey(agentSecret)
	if errorValue != nil {
		agentPubkey = ""
	}
	bootstrapSecret := buzzidentity.Secret(seed, buzzidentity.BootstrapSubject)
	return service.pickBuzzRoomActor(heldRoles, adminSecretsByPubkey, agentSecret, agentPubkey, bootstrapSecret)
}

func (service *Service) pickBuzzRoomActor(
	heldRoles map[string]string,
	adminSecretsByPubkey map[string]string,
	agentSecret string,
	agentPubkey string,
	bootstrapSecret string,
) string {
	if isElevatedBuzzRole(heldRoles[agentPubkey]) {
		return agentSecret
	}
	adminPubkeys := make([]string, 0, len(adminSecretsByPubkey))
	for pubkey := range adminSecretsByPubkey {
		adminPubkeys = append(adminPubkeys, pubkey)
	}
	sort.Strings(adminPubkeys)
	for _, pubkey := range adminPubkeys {
		if isElevatedBuzzRole(heldRoles[pubkey]) {
			log.Printf("buzz membership: the agent does not administer this room yet, signing as an administrator instead")
			return adminSecretsByPubkey[pubkey]
		}
	}
	log.Printf("buzz membership: no admin or agent administers this room yet, signing as bootstrap")
	return bootstrapSecret
}

func isElevatedBuzzRole(role string) bool {
	return role == "owner" || role == "admin"
}

func holdsAnotherAdministrator(heldRoles map[string]string, excludedPubkey string) bool {
	for pubkey, role := range heldRoles {
		if pubkey != excludedPubkey && isElevatedBuzzRole(role) {
			return true
		}
	}
	return false
}

// The company account leaves a room the moment somebody else administers it.
// Until then it stays, because it is the only key that can still seat people
// there; the member and seat syncs put an administrator in first, so the stay
// is one tick, not a policy.
func (service *Service) retireBootstrapFromRoom(ctx context.Context, relay *sql.DB, channelID string, seed string) error {
	bootstrapPubkey, errorValue := buzzPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
	if errorValue != nil {
		return errorValue
	}
	heldRoles, errorValue := buzzChannelMemberRoles(ctx, relay, channelID)
	if errorValue != nil {
		return fmt.Errorf("reading who is in %s: %w", channelID, errorValue)
	}
	if _, isHeld := heldRoles[bootstrapPubkey]; !isHeld {
		return nil
	}
	if !holdsAnotherAdministrator(heldRoles, bootstrapPubkey) {
		log.Printf("buzz membership: the company account stays in %s, nobody else administers it yet", channelID)
		return nil
	}
	if _, errorValue := removeBuzzChannelMembers(ctx, relay, channelID, []string{bootstrapPubkey}); errorValue != nil {
		return fmt.Errorf("the company account could not leave %s: %w", channelID, errorValue)
	}
	if errorValue := service.tellClientsWhoIsInTheRoom(ctx, relay, channelID); errorValue != nil {
		return fmt.Errorf("%s changed and no client was told: %w", channelID, errorValue)
	}
	log.Printf("buzz membership: the company account left %s", channelID)
	return nil
}

func (service *Service) buzzRolesByPubkey(ctx context.Context, emails []string) map[string]string {
	adminEmails := service.buzzAdminEmails(ctx)
	roles := map[string]string{}
	for _, email := range emails {
		role := buzzChannelRoleFor(adminEmails, email)
		if role == "" {
			continue
		}
		secret, errorValue := service.personBuzzSecret(ctx, email)
		if errorValue != nil {
			continue
		}
		pubkey, errorValue := buzzPublicKey(secret)
		if errorValue != nil {
			continue
		}
		roles[pubkey] = role
	}
	return roles
}

func (service *Service) ensureUserChannelMembership(ctx context.Context, email string) {
	seed := service.buzzKeySeed()
	if seed == "" || strings.TrimSpace(service.Configuration.BuzzRelayURL) == "" {
		return
	}
	secretHex := service.buzzSecretForEmail(ctx, email)
	if secretHex == "" {
		return
	}
	pubkey, errorValue := buzzPublicKey(secretHex)
	if errorValue != nil {
		return
	}
	if errorValue := service.grantRelayMembership(ctx, pubkey); errorValue != nil {
		log.Printf("buzz membership for %s: %v", email, errorValue)
	}
	channelIDs, errorValue := service.buzzStreamChannelsWeOpened(ctx)
	if errorValue != nil || len(channelIDs) == 0 {
		return
	}
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		log.Printf("buzz membership for %s: %v", email, errorValue)
		return
	}
	connections := service.newBuzzActorConnections()
	defer connections.closeAll()
	role := buzzChannelRoleFor(service.buzzAdminEmails(ctx), email)
	for _, channelID := range channelIDs {
		heldRoles, errorValue := buzzChannelMemberRoles(ctx, relay, channelID)
		if errorValue != nil {
			log.Printf("buzz membership for %s: reading who is in %s failed: %v", email, channelID, errorValue)
			continue
		}
		// The relay announces a joining in the channel for every add it is
		// asked to make, including one that changes nothing, and those
		// announcements share the fifty rows a timeline shows. An admin
		// standing in the room as an ordinary member is not nothing: the
		// add is what raises them to owner.
		if heldRole, isHeld := heldRoles[pubkey]; isHeld && (role == "" || heldRole == role) {
			continue
		}
		actorSecret := service.buzzRoomActorSecret(ctx, heldRoles, seed)
		publisher, errorValue := connections.as(ctx, actorSecret)
		if errorValue != nil {
			log.Printf("buzz membership for %s: relay connect failed: %v", email, errorValue)
			return
		}
		_ = publisher.AddMember(ctx, actorSecret, channelID, pubkey, role)
		time.Sleep(60 * time.Millisecond)
	}
}

// Everyone this company's messenger has to let in: its people, and the agent,
// which administers every room the company runs and is what a room with no
// admin in it still needs once the company account has left.
func (service *Service) everyoneTheRelayShouldHold(ctx context.Context) ([]buzzMember, error) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return nil, nil
	}
	member, errorValue := service.memberBuzzMembers(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	agentPubkey, errorValue := buzzPublicKey(buzzidentity.Secret(seed, buzzidentity.AgentSubject))
	if errorValue != nil {
		return member, nil
	}
	return append(member, buzzMember{Pubkey: agentPubkey, Role: buzzChannelOwnerRole}), nil
}

func (service *Service) ensureMemberChannelMembership(ctx context.Context) error {
	members, errorValue := service.everyoneTheRelayShouldHold(ctx)
	if errorValue != nil {
		return errorValue
	}
	admission := errors.Join(
		service.letOntoTheRelay(ctx, pubkeysOf(members)),
		service.publishTheAgentProfile(ctx),
		service.openTheAgentDirectRoomForEveryMember(ctx, members),
	)
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		return errors.Join(admission, errorValue)
	}
	return errors.Join(admission, service.seatEveryoneInTheirRooms(ctx, relay, members))
}

func (service *Service) seatEveryoneInTheirRooms(ctx context.Context, relay *sql.DB, members []buzzMember) error {
	seed := service.buzzKeySeed()
	channelIDs, errorValue := service.buzzStreamChannelsWeOpened(ctx)
	if errorValue != nil {
		return fmt.Errorf("the company's rooms could not be read: %w", errorValue)
	}
	connections := service.newBuzzActorConnections()
	defer connections.closeAll()
	return errors.Join(
		service.seatMembersInRooms(ctx, relay, connections, channelIDs, members, seed),
		service.seatTheAgentInOpenRooms(ctx, relay, connections, seed),
		service.retireBootstrapFromRemainingRooms(ctx, relay, connections, seed),
		service.removeSeatsNobodyAccountsFor(ctx, relay, channelIDs, seed),
		retirePrivateRoomsNobodyIsIn(ctx, relay),
	)
}

func (service *Service) seatMembersInRooms(
	ctx context.Context,
	relay *sql.DB,
	connections *buzzActorConnections,
	channelIDs []string,
	members []buzzMember,
	seed string,
) error {
	var refusals []error
	for _, channelID := range channelIDs {
		if errorValue := service.seatMembersInRoom(ctx, relay, connections, channelID, members, seed); errorValue != nil {
			refusals = append(refusals, fmt.Errorf("room %s: %w", channelID, errorValue))
			continue
		}
		refusals = append(refusals, service.retireBootstrapFromRoom(ctx, relay, channelID, seed))
	}
	return errors.Join(refusals...)
}

func (service *Service) seatMembersInRoom(
	ctx context.Context,
	relay *sql.DB,
	connections *buzzActorConnections,
	channelID string,
	members []buzzMember,
	seed string,
) error {
	heldRoles, errorValue := buzzChannelMemberRoles(ctx, relay, channelID)
	if errorValue != nil {
		return fmt.Errorf("reading who is in it: %w", errorValue)
	}
	actorSecret := service.buzzRoomActorSecret(ctx, heldRoles, seed)
	var refusals []error
	for _, member := range members {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		heldRole, isHeld := heldRoles[member.Pubkey]
		// The relay announces a joining in the channel for every add it is
		// asked to make, including one that changes nothing, and those
		// announcements share the fifty rows a timeline shows. An admin
		// standing in the room as an ordinary member is not nothing: the
		// add is what raises them to owner.
		if isHeld && (member.Role == "" || heldRole == member.Role) {
			continue
		}
		if errorValue := connections.addMember(ctx, actorSecret, channelID, member.Pubkey, member.Role); errorValue != nil {
			refusals = append(refusals, fmt.Errorf("seating %s: %w", member.Pubkey, errorValue))
		}
		time.Sleep(60 * time.Millisecond)
	}
	return errors.Join(refusals...)
}

func (service *Service) retireBootstrapFromRemainingRooms(ctx context.Context, relay *sql.DB, connections *buzzActorConnections, seed string) error {
	bootstrapPubkey, errorValue := buzzPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
	if errorValue != nil {
		return errorValue
	}
	channelIDs, errorValue := streamRoomsHolding(ctx, relay, bootstrapPubkey)
	if errorValue != nil {
		return fmt.Errorf("rooms still holding the company account could not be read: %w", errorValue)
	}
	var refusals []error
	for _, channelID := range channelIDs {
		refusals = append(refusals,
			service.raiseAdminsStandingInRoom(ctx, relay, connections, channelID, seed),
			service.retireBootstrapFromRoom(ctx, relay, channelID, seed))
	}
	return errors.Join(refusals...)
}

func streamRoomsHolding(ctx context.Context, relay *sql.DB, pubkey string) ([]string, error) {
	rows, errorValue := relay.QueryContext(ctx, `
SELECT m.channel_id::text FROM channel_members m
JOIN channels c ON c.id = m.channel_id AND c.community_id = m.community_id
WHERE m.pubkey = decode($1, 'hex') AND m.removed_at IS NULL
  AND c.deleted_at IS NULL AND c.channel_type = 'stream'`, pubkey)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	var channelIDs []string
	for rows.Next() {
		var channelID string
		if errorValue := rows.Scan(&channelID); errorValue != nil {
			return nil, errorValue
		}
		channelIDs = append(channelIDs, channelID)
	}
	return channelIDs, rows.Err()
}

func (service *Service) raiseAdminsStandingInRoom(ctx context.Context, relay *sql.DB, connections *buzzActorConnections, channelID string, seed string) error {
	heldRoles, errorValue := buzzChannelMemberRoles(ctx, relay, channelID)
	if errorValue != nil {
		return fmt.Errorf("reading who is in %s: %w", channelID, errorValue)
	}
	emails, errorValue := service.allMemberEmails(ctx)
	if errorValue != nil {
		return errorValue
	}
	actorSecret := service.buzzRoomActorSecret(ctx, heldRoles, seed)
	var refusals []error
	for pubkey, role := range service.buzzRolesByPubkey(ctx, emails) {
		heldRole, isHeld := heldRoles[pubkey]
		if !isHeld || heldRole == role {
			continue
		}
		if errorValue := connections.addMember(ctx, actorSecret, channelID, pubkey, role); errorValue != nil {
			refusals = append(refusals, fmt.Errorf("raising an admin in %s: %w", channelID, errorValue))
		}
		time.Sleep(60 * time.Millisecond)
	}
	return errors.Join(refusals...)
}

const memberRoomQuery = `
SELECT id FROM channels
WHERE channel_type = 'stream'
  AND deleted_at IS NULL
  AND visibility = 'open'
  AND created_by = ANY(ARRAY(SELECT decode(unnest($1::text[]), 'hex')))`

// The rooms the company runs were opened by the key that owns the relay when
// they were imported, and are opened by the agent now; a room a person opened
// is theirs, not the company's.
func (service *Service) companyRoomCreatorPubkeys() ([]string, error) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return nil, errors.New("this device holds no buzz key seed")
	}
	pubkeys := make([]string, 0, 2)
	for _, subject := range []string{buzzidentity.BootstrapSubject, buzzidentity.AgentSubject} {
		pubkey, errorValue := buzzPublicKey(buzzidentity.Secret(seed, subject))
		if errorValue != nil {
			return nil, errorValue
		}
		pubkeys = append(pubkeys, pubkey)
	}
	return pubkeys, nil
}

// The whole company belongs in the rooms the whole company can already read.
// A private room is somebody's decision about who is in it, so it is not a room
// to add everyone to.
func (service *Service) buzzStreamChannelsWeOpened(ctx context.Context) ([]string, error) {
	creatorPubkeys, errorValue := service.companyRoomCreatorPubkeys()
	if errorValue != nil {
		return nil, errorValue
	}
	database, errorValue := service.buzzDatabase()
	if errorValue != nil {
		return nil, errorValue
	}
	rows, errorValue := database.QueryContext(ctx, memberRoomQuery, pq.Array(creatorPubkeys))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	var channelIDs []string
	for rows.Next() {
		var id string
		if errorValue := rows.Scan(&id); errorValue != nil {
			continue
		}
		channelIDs = append(channelIDs, id)
	}
	return channelIDs, rows.Err()
}

// Everyone the device holds a record of, from the policy it serves and the
// company it belongs to. The record carries the name as well as the address, so
// one read answers both who is seated and what they are called.
func (service *Service) directoryRecords(ctx context.Context) []adminUserMutation {
	records := service.blueclawPolicyUserRecords(ctx)
	if found, errorValue := service.currentUserRecords(ctx); errorValue == nil {
		records = append(records, found...)
	}
	return records
}

func (service *Service) allMemberEmails(ctx context.Context) ([]string, error) {
	addresses, errorValue := service.addressesTheDirectoryHolds(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	seen := map[string]bool{}
	emails := []string{}
	for _, address := range addresses {
		email := strings.ToLower(strings.TrimSpace(address))
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		emails = append(emails, email)
	}
	return emails, nil
}

type buzzMember struct {
	Pubkey string
	Role   string
	Email  string
}

func (service *Service) memberBuzzMembers(ctx context.Context) ([]buzzMember, error) {
	emails, errorValue := service.allMemberEmails(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	adminEmails := service.buzzAdminEmails(ctx)
	seen := map[string]bool{}
	var members []buzzMember
	for _, email := range emails {
		secretHex := service.buzzSecretForEmail(ctx, email)
		if secretHex == "" {
			continue
		}
		pubkey, errorValue := buzzPublicKey(secretHex)
		if errorValue != nil || seen[pubkey] {
			continue
		}
		seen[pubkey] = true
		members = append(members, buzzMember{Pubkey: pubkey, Role: buzzChannelRoleFor(adminEmails, email), Email: email})
	}
	return members, nil
}
