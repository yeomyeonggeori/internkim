package admind

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
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

// A client lists rooms from their kind 39000 discovery events, not from the
// channels table, and reconcile-channels writes an event only where none
// exists. A row changed without dropping its event leaves every client showing
// the room as it was, through a reload and through a restart.
func buzzRoomChangeRepublish() string {
	return `
export BUZZ_RELAY_PRIVATE_KEY=$(grep '^BUZZ_RELAY_PRIVATE_KEY=' ` + blueclaw.BuzzRelayKeyEnvironmentFilePath + ` | head -1 | sed 's/^BUZZ_RELAY_PRIVATE_KEY=//')
export DATABASE_URL=$(grep '^DATABASE_URL=' ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath + ` | head -1 | sed 's/^DATABASE_URL=//')
export RELAY_URL=$(systemctl show ` + blueclaw.BuzzRelayServiceName + ` -p Environment | tr ' ' '\n' | sed -n 's/^RELAY_URL=//p' | head -1)
if [ -z "$BUZZ_RELAY_PRIVATE_KEY" ] || [ -z "$RELAY_URL" ]; then
  echo "the rows changed but no client was told: this device names no relay key or public host"
  exit 1
fi
printf '== told the clients ==\n'
` + blueclaw.BuzzAdminBinaryPath + ` reconcile-channels
`
}
