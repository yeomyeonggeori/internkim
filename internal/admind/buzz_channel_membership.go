package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
	"gitlab.com/eastriver/internkim/internal/buzzidentity"
	"gitlab.com/eastriver/internkim/internal/buzzimport/relaypublish"
)

func (service *Service) startStaffChannelMembershipSync(ctx context.Context) {
	if !service.canWriteToBuzzRelay() {
		return
	}
	go service.ensureStaffChannelMembership(ctx)
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

// Membership in a room is granted by someone who administers that room, the
// way it would be if they clicked the button themselves: an admin standing in
// it, or the agent when no admin is. The bootstrap key is the last resort that
// existing rooms still need until an admin holds owner in each; every use of
// it is a room the migration has not reached.
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
	isElevated := func(role string) bool { return role == "owner" || role == "admin" }
	adminPubkeys := make([]string, 0, len(adminSecretsByPubkey))
	for pubkey := range adminSecretsByPubkey {
		adminPubkeys = append(adminPubkeys, pubkey)
	}
	sort.Strings(adminPubkeys)
	for _, pubkey := range adminPubkeys {
		if isElevated(heldRoles[pubkey]) {
			return adminSecretsByPubkey[pubkey]
		}
	}
	if isElevated(heldRoles[agentPubkey]) {
		return agentSecret
	}
	log.Printf("buzz membership: no admin or agent administers this room yet, signing as bootstrap")
	return bootstrapSecret
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
	relay, errorValue := sql.Open("postgres", strings.TrimSpace(service.Configuration.BuzzDatabaseURL))
	if errorValue != nil {
		log.Printf("buzz membership for %s: %v", email, errorValue)
		return
	}
	defer relay.Close()
	connections := service.newBuzzActorConnections()
	defer connections.closeAll()
	role := buzzChannelRoleFor(service.buzzAdminEmails(ctx), email)
	for _, channelID := range channelIDs {
		heldRoles, errorValue := buzzChannelMemberRoles(ctx, relay, channelID)
		if errorValue != nil {
			log.Printf("buzz membership for %s: reading who is in %s failed: %v", email, channelID, errorValue)
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

func (service *Service) ensureStaffChannelMembership(ctx context.Context) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return
	}
	channelIDs, errorValue := service.buzzStreamChannelsWeOpened(ctx)
	if errorValue != nil {
		log.Printf("buzz staff membership: channel query failed: %v", errorValue)
		return
	}
	staff := service.staffBuzzMembers(ctx)
	log.Printf("buzz staff membership: %d stream channels, %d staff pubkeys", len(channelIDs), len(staff))
	if len(channelIDs) == 0 || len(staff) == 0 {
		return
	}
	connections := service.newBuzzActorConnections()
	defer connections.closeAll()
	for _, member := range staff {
		service.grantRelayMembership(ctx, member.Pubkey)
	}
	relay, errorValue := sql.Open("postgres", strings.TrimSpace(service.Configuration.BuzzDatabaseURL))
	if errorValue != nil {
		log.Printf("buzz staff membership: %v", errorValue)
		return
	}
	defer relay.Close()

	granted, failed, alreadyIn := 0, 0, 0
	for _, channelID := range channelIDs {
		heldRoles, errorValue := buzzChannelMemberRoles(ctx, relay, channelID)
		if errorValue != nil {
			log.Printf("buzz staff membership: reading who is in %s failed: %v", channelID, errorValue)
			return
		}
		actorSecret := service.buzzRoomActorSecret(ctx, heldRoles, seed)
		publisher, errorValue := connections.as(ctx, actorSecret)
		if errorValue != nil {
			log.Printf("buzz staff membership: relay connect failed: %v", errorValue)
			return
		}
		for _, member := range staff {
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
					log.Printf("buzz staff membership: relay would not take the connection back: %v", reconnectError)
					log.Printf("buzz staff membership: granted %d, failed %d", granted, failed+1)
					return
				}
				publisher = reopened
				errorValue = publisher.AddMember(ctx, actorSecret, channelID, member.Pubkey, member.Role)
			}
			if errorValue != nil {
				if failed == 0 {
					log.Printf("buzz staff membership: first AddMember error: %v", errorValue)
				}
				failed++
			} else {
				granted++
			}
			time.Sleep(60 * time.Millisecond)
		}
	}
	log.Printf("buzz staff membership: granted %d, failed %d, already in %d", granted, failed, alreadyIn)
}

func (service *Service) bootstrapBuzzPubkey() (string, error) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return "", errors.New("this device holds no buzz key seed")
	}
	return buzzPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
}

const staffRoomQuery = `
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
	database, errorValue := sql.Open("postgres", strings.TrimSpace(service.Configuration.BuzzDatabaseURL))
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, staffRoomQuery, pq.Array(creatorPubkeys), pq.Array(circleRoomNames))
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

func (service *Service) allStaffEmails(ctx context.Context) []string {
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
	for _, record := range service.blueclawPolicyUserRecords(ctx) {
		add(record.Email)
	}
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID != "" && fleetSecret != "" {
		if records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret); errorValue == nil {
			for _, record := range records {
				add(record.Email)
			}
		}
	}
	for _, email := range service.usersSyncCacheEmails() {
		add(email)
	}
	if records, errorValue := service.currentUserRecords(ctx); errorValue == nil {
		for _, record := range records {
			add(record.Email)
		}
	}
	add(service.seedAdminEmail())
	add(service.claimedAdminEmail())
	return emails
}

func (service *Service) usersSyncCacheEmails() []string {
	stateDirectory := filepath.Dir(service.stateDatabasePath())
	content, errorValue := os.ReadFile(filepath.Join(stateDirectory, "users-sync.json"))
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

type buzzStaffMember struct {
	Pubkey string
	Role   string
}

func (service *Service) staffBuzzMembers(ctx context.Context) []buzzStaffMember {
	adminEmails := service.buzzAdminEmails(ctx)
	seen := map[string]bool{}
	var members []buzzStaffMember
	for _, email := range service.allStaffEmails(ctx) {
		secretHex := service.buzzSecretForEmail(ctx, email)
		if secretHex == "" {
			continue
		}
		pubkey, errorValue := buzzPublicKey(secretHex)
		if errorValue != nil || seen[pubkey] {
			continue
		}
		seen[pubkey] = true
		members = append(members, buzzStaffMember{Pubkey: pubkey, Role: buzzChannelRoleFor(adminEmails, email)})
	}
	return members
}
