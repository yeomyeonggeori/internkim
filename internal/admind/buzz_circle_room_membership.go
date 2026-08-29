package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

type circleRoomOutcome struct {
	Room     string   `json:"room"`
	CircleID string   `json:"circleID"`
	Belong   int      `json:"belong"`
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
}

type circleRoomReport struct {
	Applied bool                `json:"applied"`
	Added   int                 `json:"added"`
	Removed int                 `json:"removed"`
	Rooms   []circleRoomOutcome `json:"rooms"`
}

const (
	circleRoomSyncInterval = 2 * time.Minute
	circleRoomSyncTimeout  = 10 * time.Minute
)

// The relay lets any member of a private room invite anybody, so a circle room
// can gain someone the circle does not hold and nothing refuses it. Removing
// them on a tick is not the same as refusing the invite, and it is what this
// device can do.
func (service *Service) startCircleRoomMembershipSync(ctx context.Context) {
	if !service.canWriteToBuzzRelay() {
		return
	}
	log.Printf("circle rooms are kept to their circles every %s", circleRoomSyncInterval)
	go func() {
		ticker := time.NewTicker(circleRoomSyncInterval)
		defer ticker.Stop()
		for {
			service.keepCircleRoomsToTheirCircles(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// A timeline shows fifty rows and a joining announcement takes one of them, so
// a room that gained members faster than it gained messages shows nobody's
// words. The relay writes an announcement for every add it is asked to make
// and has no opinion about how many a room should keep.
const joiningNoticesARoomKeeps = 10

func (service *Service) keepJoiningNoticesFromEatingTheWindow(ctx context.Context) {
	databaseURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	if databaseURL == "" {
		return
	}
	relay, errorValue := sql.Open("postgres", databaseURL)
	if errorValue != nil {
		log.Printf("joining notices could not be counted: %v", errorValue)
		return
	}
	defer relay.Close()

	result, errorValue := relay.ExecContext(ctx, `
DELETE FROM events WHERE kind = 40099 AND id IN (
  SELECT id FROM (
    SELECT id, row_number() OVER (PARTITION BY channel_id ORDER BY created_at DESC, id ASC) AS place
    FROM events WHERE kind = 40099
  ) ranked WHERE place > $1
)`, joiningNoticesARoomKeeps)
	if errorValue != nil {
		log.Printf("joining notices could not be pruned: %v", errorValue)
		return
	}
	if removed, _ := result.RowsAffected(); removed > 0 {
		log.Printf("joining notices beyond the newest %d in a room: %d taken back", joiningNoticesARoomKeeps, removed)
	}
}

func (service *Service) keepCircleRoomsToTheirCircles(ctx context.Context) {
	syncContext, cancel := context.WithTimeout(ctx, circleRoomSyncTimeout)
	defer cancel()
	service.keepJoiningNoticesFromEatingTheWindow(syncContext)
	report, errorValue := service.reconcileCircleRoomMembership(syncContext, true)
	if errorValue != nil {
		log.Printf("circle rooms could not be kept to their circles: %v", errorValue)
		return
	}
	if report.Added == 0 && report.Removed == 0 {
		return
	}
	log.Printf("circle rooms: added %d, removed %d across %d rooms", report.Added, report.Removed, len(report.Rooms))
}

func (service *Service) handleCircleRoomMembership(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "staff access required", http.StatusForbidden)
		return
	}
	report, errorValue := service.reconcileCircleRoomMembership(request.Context(), request.URL.Query().Get("apply") == "true")
	if errorValue != nil {
		log.Printf("circle room membership failed: %v", errorValue)
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(report)
}

type declaredCircle struct {
	CircleID    string
	DisplayName string
}

func declaredCirclesOfPolicy(policyDocument map[string]any) []declaredCircle {
	values, _ := policyDocument["circles"].([]any)
	circles := make([]declaredCircle, 0, len(values))
	for _, value := range values {
		circle, isCircle := value.(map[string]any)
		if !isCircle {
			continue
		}
		circleID := strings.TrimSpace(mattermostPolicyString(circle["circleID"]))
		displayName := strings.TrimSpace(mattermostPolicyString(circle["displayName"]))
		if circleID == "" || displayName == "" {
			continue
		}
		circles = append(circles, declaredCircle{CircleID: circleID, DisplayName: displayName})
	}
	return circles
}

func (service *Service) reconcileCircleRoomMembership(ctx context.Context, shouldApply bool) (circleRoomReport, error) {
	seed := service.buzzKeySeed()
	databaseURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	if seed == "" || databaseURL == "" {
		return circleRoomReport{}, errors.New("this device names no buzz key seed or database")
	}
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return circleRoomReport{}, errorValue
	}
	circlesByEmail := blueclawCirclesByEmail(policyDocument)

	relay, errorValue := sql.Open("postgres", databaseURL)
	if errorValue != nil {
		return circleRoomReport{}, errorValue
	}
	defer relay.Close()

	report := circleRoomReport{Applied: shouldApply, Rooms: []circleRoomOutcome{}}
	for _, circle := range declaredCirclesOfPolicy(policyDocument) {
		outcome, errorValue := service.reconcileOneCircleRoom(ctx, relay, seed, circle, circlesByEmail, shouldApply)
		if errorValue != nil {
			return report, errorValue
		}
		if outcome == nil {
			continue
		}
		report.Added += len(outcome.Added)
		report.Removed += len(outcome.Removed)
		report.Rooms = append(report.Rooms, *outcome)
	}
	return report, nil
}

func (service *Service) reconcileOneCircleRoom(
	ctx context.Context,
	relay *sql.DB,
	seed string,
	circle declaredCircle,
	circlesByEmail map[string][]string,
	shouldApply bool,
) (*circleRoomOutcome, error) {
	channelID, errorValue := service.circleRoomWeOpened(ctx, relay, circle.DisplayName)
	if errorValue != nil || channelID == "" {
		return nil, errorValue
	}
	belong, errorValue := service.pubkeysOf(ctx, emailsCarrying(circlesByEmail, circle.CircleID), seed)
	if errorValue != nil {
		return nil, errorValue
	}
	held, errorValue := buzzChannelMemberPubkeys(ctx, relay, channelID)
	if errorValue != nil {
		return nil, errorValue
	}
	outcome := circleRoomOutcome{Room: circle.DisplayName, CircleID: circle.CircleID, Belong: len(belong), Added: []string{}, Removed: []string{}}
	isHeld := map[string]bool{}
	for _, pubkey := range held {
		isHeld[pubkey] = true
		if !belong[pubkey] {
			outcome.Removed = append(outcome.Removed, pubkey)
		}
	}
	for pubkey := range belong {
		if !isHeld[pubkey] {
			outcome.Added = append(outcome.Added, pubkey)
		}
	}
	if !shouldApply {
		return &outcome, nil
	}
	if len(outcome.Removed) > 0 {
		if _, errorValue := removeBuzzChannelMembers(ctx, relay, channelID, outcome.Removed); errorValue != nil {
			return nil, errorValue
		}
		log.Printf("circle room %s: removed %d the circle does not hold", circle.DisplayName, len(outcome.Removed))
	}
	if len(outcome.Added) > 0 {
		roles := service.buzzRolesByPubkey(ctx, emailsCarrying(circlesByEmail, circle.CircleID))
		if errorValue := service.addToCircleRoom(ctx, seed, channelID, outcome.Added, roles); errorValue != nil {
			return nil, errorValue
		}
	}
	if errorValue := service.tellClientsWhoIsInTheRoom(ctx, relay, channelID); errorValue != nil {
		return nil, errorValue
	}
	return &outcome, nil
}

// A client reads a room's members from its kind 39002 discovery event, not from
// channel_members, so a membership this device writes is invisible until the
// event is written again. reconcile-channels writes one only where none exists,
// which is what makes dropping this room's first.
func (service *Service) tellClientsWhoIsInTheRoom(ctx context.Context, relay *sql.DB, channelID string) error {
	if _, errorValue := relay.ExecContext(ctx,
		"DELETE FROM events WHERE kind IN (39000,39001,39002) AND channel_id = $1", channelID); errorValue != nil {
		return errorValue
	}
	output, errorValue := service.runCommand(ctx, "sh", "-lc", buzzRoomChangeRepublish())
	if errorValue != nil {
		return fmt.Errorf("the room changed and no client was told: %s: %w", strings.TrimSpace(string(output)), errorValue)
	}
	return nil
}

func (service *Service) addToCircleRoom(ctx context.Context, seed string, channelID string, pubkeys []string, roles map[string]string) error {
	bootstrapSecret := buzzidentity.Secret(seed, buzzidentity.BootstrapSubject)
	publisher, errorValue := service.connectToTheRelayOnceItAnswers(ctx, bootstrapSecret)
	if errorValue != nil {
		return errorValue
	}
	defer publisher.Close()
	for _, pubkey := range pubkeys {
		if errorValue := publisher.AddMember(ctx, bootstrapSecret, channelID, pubkey, roles[pubkey]); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func emailsCarrying(circlesByEmail map[string][]string, circleID string) []string {
	emails := []string{}
	for email, circles := range circlesByEmail {
		for _, held := range circles {
			if strings.EqualFold(strings.TrimSpace(held), circleID) {
				emails = append(emails, email)
				break
			}
		}
	}
	return emails
}

func (service *Service) circleRoomWeOpened(ctx context.Context, relay *sql.DB, roomName string) (string, error) {
	bootstrapPubkey, errorValue := service.bootstrapBuzzPubkey()
	if errorValue != nil {
		return "", errorValue
	}
	var channelID string
	errorValue = relay.QueryRowContext(ctx, `
SELECT id::text FROM channels
WHERE channel_type = 'stream'
  AND deleted_at IS NULL
  AND name = $1
  AND created_by = decode($2, 'hex')`, roomName, bootstrapPubkey).Scan(&channelID)
	if errorValue == sql.ErrNoRows {
		return "", nil
	}
	return channelID, errorValue
}

func (service *Service) circleRoomNames(ctx context.Context) ([]string, error) {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return nil, errorValue
	}
	circles := declaredCirclesOfPolicy(policyDocument)
	names := make([]string, 0, len(circles))
	for _, circle := range circles {
		names = append(names, circle.DisplayName)
	}
	return names, nil
}

func (service *Service) bootstrapPubkeyOrEmpty() string {
	pubkey, errorValue := service.bootstrapBuzzPubkey()
	if errorValue != nil {
		return ""
	}
	return pubkey
}
