package setup

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestDeliveryShareIsBoundReadOnlyBeforeBlueclawStarts(t *testing.T) {
	mountCommand := buildBlueclawDeliveryMountCommand()

	for _, fragment := range []string{
		"mount --bind " + blueclaw.BlueclawDeliveryPath + " " + blueclaw.BlueclawDeliveryReadOnlyPath,
		"mount -o remount,bind,ro " + blueclaw.BlueclawDeliveryReadOnlyPath,
		"Before=blueclaw.service",
	} {
		if !strings.Contains(mountCommand, fragment) {
			t.Fatalf("virtiofsd has no read-only mode, so the bind must carry it: missing %q", fragment)
		}
	}

	if strings.Index(mountCommand, "mount --bind") > strings.Index(mountCommand, "remount,bind,ro") {
		t.Fatal("a bind takes its flags on the remount, so the bind has to come first")
	}
	if !strings.Contains(mountCommand, "findmnt -no OPTIONS") {
		t.Fatal("the unit must prove the share came up read-only rather than report success for starting")
	}
	if strings.Contains(blueclaw.BlueclawDeliveryServiceName, `\`) {
		t.Fatal("a unit named after a path needs systemd's escaping, which is a class of bug worth not having")
	}
}

func TestDeliveredConfigurationIsWrittenWhereTheShareServesIt(t *testing.T) {
	if !strings.Contains(buildBlueclawConfigurationDirectoryCommand(), blueclaw.BlueclawDeliveryConfigPath) {
		t.Fatal("expected the delivered configuration directory to be created")
	}
}
