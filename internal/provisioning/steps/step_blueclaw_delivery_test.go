package setup

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestDeliveryShareIsBoundReadOnlyBeforeBlueclawStarts(t *testing.T) {
	mountCommand := buildBlueclawDeliveryMountCommand()

	for _, fragment := range []string{
		"Options=bind,ro",
		"What=" + blueclaw.BlueclawDeliveryPath,
		"Where=" + blueclaw.BlueclawDeliveryReadOnlyPath,
		"Before=blueclaw.service",
	} {
		if !strings.Contains(mountCommand, fragment) {
			t.Fatalf("virtiofsd has no read-only mode, so the bind must carry it: missing %q", fragment)
		}
	}
}

func TestDeliveredConfigurationIsWrittenWhereTheShareServesIt(t *testing.T) {
	if !strings.Contains(buildBlueclawConfigurationDirectoryCommand(), blueclaw.BlueclawDeliveryConfigPath) {
		t.Fatal("expected the delivered configuration directory to be created")
	}
}
