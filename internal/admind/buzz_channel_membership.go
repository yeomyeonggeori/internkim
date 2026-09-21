package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
	"gitlab.com/eastriver/internkim/internal/buzzidentity"
	"gitlab.com/eastriver/internkim/internal/buzzimport/relaypublish"
)

// Rooms drift between passes: somebody joins the company, somebody leaves it,
// and a profile the directory could have written never got written. One pass a
// day keeps the drift to a day.
const memberChannelSyncInterval = 24 * time.Hour

func (service *Service) startMemberChannelMembershipSync(ctx context.Context) {
	if !service.canWriteToBuzzRelay() {
		return
	}
	go func() {
		ticker := time.NewTicker(memberChannelSyncInterval)
		defer ticker.Stop()
		for {
			service.withinASweepBudget(ctx, service.ensureMemberChannelMembership)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (service *Service) grantRelayMembership(ctx context.Context, pubkey string) {
	command := strings.TrimSpace(service.Configuration.BuzzAdminCommandPath)
	databaseURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	if command == "" || databaseURL == "" {
		return
	}
	execution := exec.CommandContext(ctx, command, "add-member", "--pubkey", pubkey)
	execution.Env = append(os.Environ(), "DATABASE_URL="+databaseURL, "RELAY_URL="+service.buzzRelayEffectiveURL())
	if content, errorValue := os.ReadFile(strings.TrimSpace(service.Configuration.BuzzRelayKeyPath)); errorValue == nil {
		for _, line := range strings.Split(string(content), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "BUZZ_RELAY_PRIVATE_KEY=") {
				execution.Env = append(execution.Env, strings.TrimSpace(line))
			}
		}
	}
	if output, errorValue := execution.CombinedOutput(); errorValue != nil && !strings.Contains(string(output), "already") {
		log.Printf("buzz relay membership grant for %s failed: %v (%s)", pubkey, errorValue, strings.TrimSpace(string(output)))
	}
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
		publisher, errorValue := relaypublish.Connect(ctx, service.buzzRelayEffectiveURL(), actorSecretHex)
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
// there; the member and circle syncs put an administrator in first, so the stay
// is one tick, not a policy.
func (service *Service) retireBootstrapFromRoom(ctx context.Context, relay *sql.DB, channelID string, seed string) {
	bootstrapPubkey, errorValue := buzzPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
	if errorValue != nil {
		return
	}
	heldRoles, errorValue := buzzChannelMemberRoles(ctx, relay, channelID)
	if errorValue != nil {
		log.Printf("buzz membership: reading who is in %s failed: %v", channelID, errorValue)
		return
	}
	if _, isHeld := heldRoles[bootstrapPubkey]; !isHeld {
		return
	}
	if !holdsAnotherAdministrator(heldRoles, bootstrapPubkey) {
		log.Printf("buzz membership: the company account stays in %s, nobody else administers it yet", channelID)
		return
	}
	if _, errorValue := removeBuzzChannelMembers(ctx, relay, channelID, []string{bootstrapPubkey}); errorValue != nil {
		log.Printf("buzz membership: the company account could not leave %s: %v", channelID, errorValue)
		return
	}
	if errorValue := service.tellClientsWhoIsInTheRoom(ctx, relay, channelID); errorValue != nil {
		log.Printf("buzz membership: %s changed and no client was told: %v", channelID, errorValue)
		return
	}
	log.Printf("buzz membership: the company account left %s", channelID)
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
	service.grantRelayMembership(ctx, pubkey)
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

func (service *Service) ensureMemberChannelMembership(ctx context.Context) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return
	}
	member := service.memberBuzzMembers(ctx)
	// The agent administers every room the company runs, which is what a room
	// with no admin in it still needs once the company account has left.
	if agentPubkey, errorValue := buzzPublicKey(buzzidentity.Secret(seed, buzzidentity.AgentSubject)); errorValue == nil {
		member = append(member, buzzMember{Pubkey: agentPubkey, Role: buzzChannelOwnerRole})
	}
	for _, member := range member {
		service.grantRelayMembership(ctx, member.Pubkey)
	}
	channelIDs, errorValue := service.buzzStreamChannelsWeOpened(ctx)
	if errorValue != nil {
		log.Printf("buzz member membership: channel query failed: %v", errorValue)
		return
	}
	log.Printf("buzz member membership: %d stream channels, %d member pubkeys", len(channelIDs), len(member))
	if len(channelIDs) == 0 || len(member) == 0 {
		return
	}
	connections := service.newBuzzActorConnections()
	defer connections.closeAll()
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		log.Printf("buzz member membership: %v", errorValue)
		return
	}

	granted, failed, alreadyIn := 0, 0, 0
	for _, channelID := range channelIDs {
		heldRoles, errorValue := buzzChannelMemberRoles(ctx, relay, channelID)
		if errorValue != nil {
			log.Printf("buzz member membership: reading who is in %s failed: %v", channelID, errorValue)
			return
		}
		actorSecret := service.buzzRoomActorSecret(ctx, heldRoles, seed)
		publisher, errorValue := connections.as(ctx, actorSecret)
		if errorValue != nil {
			log.Printf("buzz member membership: relay connect failed: %v", errorValue)
			return
		}
		for _, member := range member {
			select {
			case <-ctx.Done():
				return
			default:
			}
			heldRole, isHeld := heldRoles[member.Pubkey]
			// The relay announces a joining in the channel for every add it is
			// asked to make, including one that changes nothing, and those
			// announcements share the fifty rows a timeline shows. An admin
			// standing in the room as an ordinary member is not nothing: the
			// add is what raises them to owner.
			if isHeld && (member.Role == "" || heldRole == member.Role) {
				alreadyIn++
				continue
			}
			errorValue := publisher.AddMember(ctx, actorSecret, channelID, member.Pubkey, member.Role)
			// The relay drops a connection that only publishes, so one that has
			// been dropped is opened again rather than every remaining grant
			// failing against it.
			if errorValue != nil && strings.Contains(errorValue.Error(), "connection closed") {
				connections.drop(actorSecret)
				reopened, reconnectError := connections.as(ctx, actorSecret)
				if reconnectError != nil {
					log.Printf("buzz member membership: relay would not take the connection back: %v", reconnectError)
					log.Printf("buzz member membership: granted %d, failed %d", granted, failed+1)
					return
				}
				publisher = reopened
				errorValue = publisher.AddMember(ctx, actorSecret, channelID, member.Pubkey, member.Role)
			}
			if errorValue != nil {
				if failed == 0 {
					log.Printf("buzz member membership: first AddMember error: %v", errorValue)
				}
				failed++
			} else {
				granted++
			}
			time.Sleep(60 * time.Millisecond)
		}
		service.retireBootstrapFromRoom(ctx, relay, channelID, seed)
	}
	log.Printf("buzz member membership: granted %d, failed %d, already in %d", granted, failed, alreadyIn)
	service.retireBootstrapFromRemainingRooms(ctx, relay, connections, seed)
	service.nameMembersTheRelayCannotName(ctx, relay)
	service.removeSeatsNobodyAccountsFor(ctx, relay, channelIDs, seed)
}

// The member and circle syncs cover the rooms the company runs, but the company
// account also stands in rooms it only mirrored — private rooms whose members
// it seated. It leaves those the same way: any admin standing in the room is
func (service *Service) retireBootstrapFromRemainingRooms(ctx context.Context, relay *sql.DB, connections *buzzActorConnections, seed string) {
	bootstrapSecret := buzzidentity.Secret(seed, buzzidentity.BootstrapSubject)
	bootstrapPubkey, errorValue := buzzPublicKey(bootstrapSecret)
	if errorValue != nil {
		return
	}
	rows, errorValue := relay.QueryContext(ctx, `
SELECT m.channel_id::text FROM channel_members m
JOIN channels c ON c.id = m.channel_id AND c.community_id = m.community_id
WHERE m.pubkey = decode($1, 'hex') AND m.removed_at IS NULL
  AND c.deleted_at IS NULL AND c.channel_type = 'stream'`, bootstrapPubkey)
	if errorValue != nil {
		log.Printf("buzz membership: rooms still holding the company account could not be read: %v", errorValue)
		return
	}
	defer rows.Close()
	var channelIDs []string
	for rows.Next() {
		var channelID string
		if errorValue := rows.Scan(&channelID); errorValue != nil {
			return
		}
		channelIDs = append(channelIDs, channelID)
	}
	for _, channelID := range channelIDs {
		service.raiseAdminsStandingInRoom(ctx, relay, connections, channelID, seed)
		service.retireBootstrapFromRoom(ctx, relay, channelID, seed)
	}
}

func (service *Service) raiseAdminsStandingInRoom(ctx context.Context, relay *sql.DB, connections *buzzActorConnections, channelID string, seed string) {
	heldRoles, errorValue := buzzChannelMemberRoles(ctx, relay, channelID)
	if errorValue != nil {
		return
	}
	adminRoles := service.buzzRolesByPubkey(ctx, service.allMemberEmails(ctx))
	actorSecret := service.buzzRoomActorSecret(ctx, heldRoles, seed)
	for pubkey, role := range adminRoles {
		heldRole, isHeld := heldRoles[pubkey]
		if !isHeld || heldRole == role {
			continue
		}
		publisher, errorValue := connections.as(ctx, actorSecret)
		if errorValue != nil {
			return
		}
		if errorValue := publisher.AddMember(ctx, actorSecret, channelID, pubkey, role); errorValue != nil {
			log.Printf("buzz membership: raising an admin in %s failed: %v", channelID, errorValue)
		}
		time.Sleep(60 * time.Millisecond)
	}
}

func (service *Service) bootstrapBuzzPubkey() (string, error) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return "", errors.New("this device holds no buzz key seed")
	}
	return buzzPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
}

const memberRoomQuery = `
SELECT id FROM channels
WHERE channel_type = 'stream'
  AND deleted_at IS NULL
  AND visibility = 'open'
  AND created_by = ANY(ARRAY(SELECT decode(unnest($1::text[]), 'hex')))
  AND name <> ALL($2::text[])`

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
// A private room is somebody's decision about who is in it, and a circle room
// is its circle's, so neither is a room to add everyone to.
func (service *Service) buzzStreamChannelsWeOpened(ctx context.Context) ([]string, error) {
	creatorPubkeys, errorValue := service.companyRoomCreatorPubkeys()
	if errorValue != nil {
		return nil, errorValue
	}
	circleRoomNames, errorValue := service.circleRoomNames(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	database, errorValue := service.buzzDatabase()
	if errorValue != nil {
		return nil, errorValue
	}
	rows, errorValue := database.QueryContext(ctx, memberRoomQuery, pq.Array(creatorPubkeys), pq.Array(circleRoomNames))
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

func (service *Service) allMemberEmails(ctx context.Context) []string {
	seen := map[string]bool{}
	var emails []string
	add := func(email string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" || seen[email] {
			return
		}
		seen[email] = true
		emails = append(emails, email)
	}
	for _, record := range service.directoryRecords(ctx) {
		add(record.Email)
	}
	for _, email := range service.usersSyncCacheEmails() {
		add(email)
	}
	add(service.seedAdminEmail())
	add(service.claimedAdminEmail())
	return emails
}

func (service *Service) usersSyncCacheEmails() []string {
	content, errorValue := os.ReadFile(service.Configuration.UsersSyncStatePath)
	if errorValue != nil {
		return nil
	}
	var cache struct {
		Users []string `json:"users"`
	}
	if json.Unmarshal(content, &cache) != nil {
		return nil
	}
	return cache.Users
}

type buzzMember struct {
	Pubkey string
	Role   string
}

func (service *Service) memberBuzzMembers(ctx context.Context) []buzzMember {
	adminEmails := service.buzzAdminEmails(ctx)
	seen := map[string]bool{}
	var members []buzzMember
	for _, email := range service.allMemberEmails(ctx) {
		secretHex := service.buzzSecretForEmail(ctx, email)
		if secretHex == "" {
			continue
		}
		pubkey, errorValue := buzzPublicKey(secretHex)
		if errorValue != nil || seen[pubkey] {
			continue
		}
		seen[pubkey] = true
		members = append(members, buzzMember{Pubkey: pubkey, Role: buzzChannelRoleFor(adminEmails, email)})
	}
	return members
}
