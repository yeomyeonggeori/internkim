package admind

import (
	"context"
	"log"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The relay admits any key that can sign unless it is told to check the
// membership list, and buzz-membership-recover turns that check off so the
// company's own keys can be put back. One company ran open for a month after a
// recovery in July: a client that brings its own key was admitted, published a
// profile, and became a second person in the messenger nobody could reach.
//
// A door held open by a drop-in that nothing takes back is worth saying out
// loud on every start, because the unit file on disk still reads closed and
// nobody looking at it would know.
func (service *Service) sayIfTheRelayIsOpen(ctx context.Context) {
	if strings.TrimSpace(service.Configuration.BuzzDatabaseURL) == "" {
		return
	}
	output, errorValue := service.runCommand(ctx, "systemctl", "show",
		blueclaw.BuzzRelayServiceName, "-p", "Environment")
	if errorValue != nil {
		return
	}
	if !strings.Contains(string(output), "BUZZ_REQUIRE_RELAY_MEMBERSHIP=false") {
		return
	}
	log.Printf("the messenger relay is running open: BUZZ_REQUIRE_RELAY_MEMBERSHIP=false admits any key that can sign, "+
		"whoever holds it. A recovery leaves this behind. Close it with: internkim recover --action buzz-membership-close")
}
