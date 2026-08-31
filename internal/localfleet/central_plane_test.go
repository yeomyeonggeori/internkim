package localfleet

import (
	"strings"
	"testing"
)

func fleetForTest(t *testing.T) Service {
	t.Helper()
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repository",
		ExecutablePath:     "/repository/internkim",
		StateRootPath:      "/state",
		VirtualMachineName: "internkim-local-fleet",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return service
}

func TestTheDeviceIsToldWhichCompanyItBelongsTo(t *testing.T) {
	command := fleetForTest(t).joinCentralPlaneCommand()

	if !strings.Contains(command, "/state/central-plane.env") {
		t.Fatalf("the settings the plane wrote are read back, got %q", command)
	}
	for _, setting := range centralPlaneDeviceSettings {
		if !strings.Contains(command, setting.path) {
			t.Fatalf("%s is not written to the device: %q", setting.variable, command)
		}
		if !strings.Contains(command, setting.variable+"=\"$"+setting.variable+"\"") {
			t.Fatalf("%s is not carried to the device: %q", setting.variable, command)
		}
	}
}

// The agent key is a secret, and a device that leaves it world readable has
// handed the company to anyone with a shell on it.
func TestTheAgentKeyLandsUnreadableToAnybodyElse(t *testing.T) {
	command := fleetForTest(t).joinCentralPlaneCommand()

	if !strings.Contains(command, "chmod 600 /root/.internkim/secrets/central-plane-agent-key") {
		t.Fatalf("the agent key is not kept to itself: %q", command)
	}
}

func TestTheDeviceReachesTheCompanyOverTheSameSession(t *testing.T) {
	service := fleetForTest(t)
	command := service.startTunnelCommand()

	if !strings.Contains(command, "-R") {
		t.Fatalf("nothing carries the company back to the device: %q", command)
	}
	if !strings.Contains(command, "127.0.0.1:54321:127.0.0.1:54321") {
		t.Fatalf("the record is not reachable from the device: %q", command)
	}
	if !strings.Contains(command, "5183:127.0.0.1:5183") {
		t.Fatalf("the app is not reachable from the device: %q", command)
	}
}

func TestBringingTheFleetUpBringsTheCompanyUpFirst(t *testing.T) {
	plans := fleetForTest(t).upPlans(false)

	planeIndex, setupIndex, joinIndex := -1, -1, -1
	for index, plan := range plans {
		text := plan.Name + " " + strings.Join(plan.Arguments, " ")
		switch {
		case strings.Contains(text, "start-local-fleet-central-plane"):
			planeIndex = index
		case strings.Contains(text, "central-plane.env"):
			joinIndex = index
		// Bringing up the reusable runtime base also runs setup, so the fleet's
		// own setup is the one that forces every step.
		case strings.Contains(text, "setup --board lab") && strings.Contains(text, "--force"):
			setupIndex = index
		}
	}
	if planeIndex < 0 || joinIndex < 0 || setupIndex < 0 {
		t.Fatalf("plane=%d join=%d setup=%d", planeIndex, joinIndex, setupIndex)
	}
	if !(planeIndex < joinIndex && joinIndex < setupIndex) {
		t.Fatalf("the company exists, then the device joins it, then setup runs: plane=%d join=%d setup=%d", planeIndex, joinIndex, setupIndex)
	}
}
