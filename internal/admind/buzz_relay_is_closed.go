package admind

import (
	"context"
	"log"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

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
		"whoever holds it. Close it by removing the drop-in that sets it under /etc/systemd/system/%s.service.d/ and restarting %s",
		blueclaw.BuzzRelayServiceName, blueclaw.BuzzRelayServiceName)
}
