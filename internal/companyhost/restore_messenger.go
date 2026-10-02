package companyhost

import (
	"fmt"
	"io"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The relay keys a community by the authority of its RELAY_URL and makes a new,
// empty one for an authority it has no row for. A device served its community
// at whatever public host it was given, and an earlier host package at its
// loopback address, so the company's one community is moved to its messenger
// host before the relay starts under that name.
func messengerCommunityRehoming(host string) string {
	literal := "'" + strings.ReplaceAll(host, "'", "''") + "'"
	return `\connect ` + blueclaw.BuzzRelayDatabaseName + `
DO $rehome$
BEGIN
  IF to_regclass('public.communities') IS NULL THEN
    RETURN;
  END IF;
  UPDATE communities SET host = ` + literal + `
   WHERE (SELECT count(*) FROM communities) = 1
     AND NOT EXISTS (SELECT 1 FROM communities WHERE lower(host) = lower(` + literal + `));
END
$rehome$;
`
}

func rehomeTheMessengerCommunity(platform companyHostPlatform, machine Machine, connection Connection, progress io.Writer) error {
	host := MessengerHost(connection)
	if errorValue := platform.RunDatabaseStatements(machine, messengerCommunityRehoming(host), progress); errorValue != nil {
		return fmt.Errorf("the messenger's community could not be moved to %s, the name its apps dial: %w", host, errorValue)
	}
	return nil
}
