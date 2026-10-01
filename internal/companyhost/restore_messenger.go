package companyhost

import (
	"fmt"
	"io"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The relay keys a community by the authority of its RELAY_URL and makes a new,
// empty one for an authority it has no row for. A device served its community
// at a public host, so the company's one community is moved to the address this
// host's relay answers at before the relay first starts.
func messengerCommunityRehoming() string {
	host := blueclaw.BuzzRelayBindAddress
	return `\connect ` + blueclaw.BuzzRelayDatabaseName + `
DO $rehome$
BEGIN
  IF to_regclass('public.communities') IS NULL THEN
    RETURN;
  END IF;
  UPDATE communities SET host = '` + host + `'
   WHERE (SELECT count(*) FROM communities) = 1
     AND NOT EXISTS (SELECT 1 FROM communities WHERE lower(host) = lower('` + host + `'));
END
$rehome$;
`
}

func rehomeTheMessengerCommunity(platform backupPlatform, machine Machine, progress io.Writer) error {
	if errorValue := platform.RunDatabaseStatements(machine, messengerCommunityRehoming(), progress); errorValue != nil {
		return fmt.Errorf("the messenger's community could not be moved to %s, where this host's relay answers: %w", blueclaw.BuzzRelayBindAddress, errorValue)
	}
	return nil
}
