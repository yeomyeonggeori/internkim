package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

const joiningNoticePruneInterval = 2 * time.Minute

func (service *Service) startJoiningNoticePruning(ctx context.Context) {
	if !service.canWriteToBuzzRelay() {
		return
	}
	go func() {
		ticker := time.NewTicker(joiningNoticePruneInterval)
		defer ticker.Stop()
		for {
			service.withinASweepBudget(ctx, service.keepJoiningNoticesFromEatingTheWindow)
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
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		log.Printf("joining notices could not be counted: %v", errorValue)
		return
	}

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

var errNoClientCanBeTold = errors.New("this host names no buzz-admin, relay database or relay key, so the room's discovery events are left as they are")

// A client reads a room's members from its kind 39002 discovery event, not from
// channel_members, so a membership this device writes is invisible until the
// event is written again. reconcile-channels writes one only where none exists,
// which is what makes dropping this room's first. A client lists rooms from
// their kind 39000 event too, so dropping it without writing it again takes the
// room off every client: nothing is dropped until the rewrite can run.
func (service *Service) tellClientsWhoIsInTheRoom(ctx context.Context, relay *sql.DB, channelID string) error {
	republish := service.buzzAdminCommand(ctx, "reconcile-channels")
	if republish == nil || service.buzzRelayKeySetting() == "" {
		return errNoClientCanBeTold
	}
	if _, errorValue := relay.ExecContext(ctx,
		"DELETE FROM events WHERE kind IN (39000,39001,39002) AND channel_id = $1", channelID); errorValue != nil {
		return errorValue
	}
	if output, errorValue := republish.CombinedOutput(); errorValue != nil {
		return fmt.Errorf("room %s changed and its discovery events were not written again, so no client lists it: %s: %w",
			channelID, strings.TrimSpace(string(output)), errorValue)
	}
	return nil
}
