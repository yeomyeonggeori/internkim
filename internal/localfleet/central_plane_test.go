package localfleet

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func joinedCentralPlaneScript(t *testing.T, command string) string {
	t.Helper()
	_, payload, isCarried := strings.Cut(command, "echo ")
	if !isCarried {
		t.Fatalf("the join carries no script: %q", command)
	}
	encoded, _, isTerminated := strings.Cut(payload, " | base64 -d")
	if !isTerminated {
		t.Fatalf("the join carries no script: %q", command)
	}
	script, errorValue := base64.StdEncoding.DecodeString(encoded)
	if errorValue != nil {
		t.Fatalf("the script the device runs is not readable: %v", errorValue)
	}
	return string(script)
}

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
	script := joinedCentralPlaneScript(t, command)

	if !strings.Contains(command, "/state/central-plane.env") {
		t.Fatalf("the settings the plane wrote are read back, got %q", command)
	}
	for _, setting := range centralPlaneDeviceSettings {
		if !strings.Contains(script, setting.path) {
			t.Fatalf("%s is not written to the device: %q", setting.variable, script)
		}
		if !strings.Contains(command, "$"+setting.variable) {
			t.Fatalf("%s is not carried to the device: %q", setting.variable, command)
		}
	}
}

// The agent key is a secret, and a device that leaves it world readable has
// handed the company to anyone with a shell on it.
func TestTheAgentKeyLandsUnreadableToAnybodyElse(t *testing.T) {
	command := fleetForTest(t).joinCentralPlaneCommand()

	script := joinedCentralPlaneScript(t, command)

	if !strings.Contains(script, "chmod 600 /root/.internkim/secrets/central-plane-agent-key") {
		t.Fatalf("the agent key is not kept to itself: %q", script)
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

func TestJoinCentralPlaneCarriesEveryValueByteForByte(t *testing.T) {
	stateDirectory := t.TempDir()
	capturePath := filepath.Join(stateDirectory, "captured-remote-command")
	stubPath := filepath.Join(stateDirectory, "internkim-stub")
	stubScript := "#!/bin/sh\nfor argument in \"$@\"; do printf '%s\\n' \"$argument\"; done > " +
		"'" + capturePath + "'\n"
	if errorValue := os.WriteFile(stubPath, []byte(stubScript), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}

	settings := map[string]string{
		"CENTRAL_PLANE_APP_URL":         "http://127.0.0.1:5183",
		"CENTRAL_PLANE_PROJECT_URL":     "http://127.0.0.1:54321",
		"CENTRAL_PLANE_PUBLISHABLE_KEY": `a key with "quotes", spaces and a $dollar`,
		"CENTRAL_PLANE_AGENT_KEY":       "agent-key-9rrfolb86o61",
	}
	environmentLines := make([]string, 0, len(settings))
	for variable, value := range settings {
		environmentLines = append(environmentLines, variable+"='"+value+"'")
	}
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     stubPath,
		StateRootPath:      stateDirectory,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	settingsBody := []byte(strings.Join(environmentLines, "\n") + "\n")
	if errorValue := os.WriteFile(service.centralPlaneSettingsPath(), settingsBody, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	output, errorValue := exec.Command("sh", "-c", service.joinCentralPlaneCommand()).CombinedOutput()
	if errorValue != nil {
		t.Fatalf("the join command did not run: %v\n%s", errorValue, output)
	}
	captured, errorValue := os.ReadFile(capturePath)
	if errorValue != nil {
		t.Fatalf("the stub captured nothing: %v", errorValue)
	}

	remoteCommand := string(captured)
	for variable, value := range settings {
		pattern := regexp.MustCompile(variable + `_BASE64=([A-Za-z0-9+/=]+)`)
		match := pattern.FindStringSubmatch(remoteCommand)
		if match == nil {
			t.Errorf("%s never left for the guest:\n%s", variable, remoteCommand)
			continue
		}
		decoded, errorValue := base64.StdEncoding.DecodeString(match[1])
		if errorValue != nil {
			t.Errorf("%s arrives undecodable: %v", variable, errorValue)
			continue
		}
		if string(decoded) != value {
			t.Errorf("%s arrives as %q, the company gave %q", variable, decoded, value)
		}
	}
}
