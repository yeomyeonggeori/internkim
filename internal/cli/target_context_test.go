package cli

import (
	"os"
	"path/filepath"
	"testing"

	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
)

func TestResolveCommandTargetDefaultsToJetsonState(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	baseStateDir := filepath.Join(homeDirectory, ".internkim")
	if errorValue := os.MkdirAll(baseStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected base state dir: %v", errorValue)
	}
	saveState(baseStateDir, "board_ip", "192.0.2.40")

	target := resolveCommandTarget(nil)
	expectedStateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)

	if target.boardType != setup.BoardJetsonOrinNano {
		t.Fatalf("expected default board %q, got %q", setup.BoardJetsonOrinNano, target.boardType)
	}
	if target.mode != commandTargetModePhysical {
		t.Fatalf("expected physical target, got %q", target.mode)
	}
	if target.stateDir != expectedStateDir {
		t.Fatalf("expected Jetson state dir %q, got %q", expectedStateDir, target.stateDir)
	}
	if loadState(target.stateDir, "board_ip") != "" {
		t.Fatalf("expected top-level board_ip not to migrate into Jetson state")
	}
}

func TestResolveCommandTargetKeepsLabExplicit(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	target := resolveCommandTarget([]string{"--board", "lab"})
	expectedStateDir := filepath.Join(homeDirectory, ".internkim", "devices", "lab")

	if target.mode != commandTargetModeLab {
		t.Fatalf("expected lab target, got %q", target.mode)
	}
	if target.stateDir != expectedStateDir {
		t.Fatalf("expected lab state dir %q, got %q", expectedStateDir, target.stateDir)
	}
}

func TestResolveCommandTargetKeepsSimulationOutOfDeviceState(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	target := resolveCommandTarget([]string{"--sim"})
	expectedStateDir := filepath.Join(homeDirectory, ".internkim", "simulations", "sim")

	if target.mode != commandTargetModeSimulation {
		t.Fatalf("expected simulation target, got %q", target.mode)
	}
	if target.stateDir != expectedStateDir {
		t.Fatalf("expected simulation state dir %q, got %q", expectedStateDir, target.stateDir)
	}
}

func TestResolveCommandTargetReadsCloudflareSSHFlag(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	stateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)
	if errorValue := os.MkdirAll(stateDir, 0o700); errorValue != nil {
		t.Fatalf("expected state dir: %v", errorValue)
	}
	saveState(stateDir, "ssh_hostname", "ssh.device.example.test")

	target := resolveCommandTarget([]string{"--cloudflare-ssh"})

	if !target.useRemoteSSH {
		t.Fatalf("expected Cloudflare SSH flag")
	}
	if target.sshHostname != "ssh.device.example.test" {
		t.Fatalf("expected SSH hostname from state, got %q", target.sshHostname)
	}
}

func TestResolveCommandTargetUsesNodeScopedFleetState(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	baseStateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)
	if errorValue := os.MkdirAll(baseStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected base state dir: %v", errorValue)
	}
	saveState(baseStateDir, "fleet_id", "fleet-one")
	saveState(baseStateDir, "fleet_secret", "fleet-secret")

	target := resolveCommandTarget([]string{"--node", "Board-B"})
	expectedStateDir := filepath.Join(baseStateDir, "boards", "board-b")

	if target.stateDir != expectedStateDir {
		t.Fatalf("expected board scoped state dir %q, got %q", expectedStateDir, target.stateDir)
	}
	if loadState(target.stateDir, "node_id") != "board-b" {
		t.Fatalf("expected node id to be stored")
	}
	if loadState(target.stateDir, "fleet_id") != "fleet-one" {
		t.Fatalf("expected fleet id to be copied")
	}
	if loadState(target.stateDir, "fleet_secret") != "fleet-secret" {
		t.Fatalf("expected fleet secret to be copied")
	}
	if target.nodeID != "board-b" {
		t.Fatalf("expected node id, got %q", target.nodeID)
	}
}

func TestResolveCommandTargetAcceptsExplicitFleetJoin(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	target := resolveCommandTarget([]string{
		"--node",
		"board-c",
		"--fleet",
		"fleet-two",
		"--fleet-secret",
		"join-secret",
	})

	if loadState(target.stateDir, "fleet_id") != "fleet-two" {
		t.Fatalf("expected explicit fleet id")
	}
	if loadState(target.stateDir, "fleet_secret") != "join-secret" {
		t.Fatalf("expected explicit fleet secret")
	}
}

func TestResolveCommandTargetNormalizesNodeID(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	target := resolveCommandTarget([]string{"--node", "Node_B"})

	if loadState(target.stateDir, "node_id") != "node-b" {
		t.Fatalf("expected DNS-safe node id, got %q", loadState(target.stateDir, "node_id"))
	}
}

func TestResolveCommandTargetDefaultsToSavedDefaultNode(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	baseStateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)
	nodeStateDir := filepath.Join(baseStateDir, "boards", "node-a")
	if errorValue := os.MkdirAll(nodeStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected node state dir: %v", errorValue)
	}
	saveState(baseStateDir, "default_node_id", "node-a")
	saveState(baseStateDir, "fleet_id", "fleet-one")
	saveState(nodeStateDir, "node_id", "node-a")
	saveState(nodeStateDir, "fleet_role", "active")

	target := resolveCommandTarget(nil)

	if target.stateDir != nodeStateDir {
		t.Fatalf("expected default node state dir %q, got %q", nodeStateDir, target.stateDir)
	}
	if target.nodeID != "node-a" {
		t.Fatalf("expected node-a, got %q", target.nodeID)
	}
}

func TestResolveCommandTargetDefaultsToFirstActiveNode(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	baseStateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)
	activeStateDir := filepath.Join(baseStateDir, "boards", "node-a")
	pendingStateDir := filepath.Join(baseStateDir, "boards", "node-b")
	if errorValue := os.MkdirAll(activeStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected active node state dir: %v", errorValue)
	}
	if errorValue := os.MkdirAll(pendingStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected pending node state dir: %v", errorValue)
	}
	saveState(activeStateDir, "node_id", "node-a")
	saveState(activeStateDir, "fleet_role", "active")
	saveState(pendingStateDir, "node_id", "node-b")
	saveState(pendingStateDir, "fleet_role", "pending")

	target := resolveCommandTarget(nil)

	if target.stateDir != activeStateDir {
		t.Fatalf("expected active node state dir %q, got %q", activeStateDir, target.stateDir)
	}
}

func TestResolveCommandTargetHostKeepsBaseStateWithoutExplicitNode(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	baseStateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)
	nodeStateDir := filepath.Join(baseStateDir, "boards", "node-a")
	if errorValue := os.MkdirAll(nodeStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected node state dir: %v", errorValue)
	}
	saveState(baseStateDir, "default_node_id", "node-a")
	saveState(nodeStateDir, "node_id", "node-a")
	saveState(nodeStateDir, "fleet_role", "active")

	target := resolveCommandTarget([]string{"--host", "192.0.2.10"})

	if target.stateDir != baseStateDir {
		t.Fatalf("expected base state dir %q, got %q", baseStateDir, target.stateDir)
	}
}

func TestResolveCommandTargetFindsStateByAssignedNodeID(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	baseStateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)
	legacyStateDir := filepath.Join(baseStateDir, "boards", "old-node")
	if errorValue := os.MkdirAll(legacyStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected node state dir: %v", errorValue)
	}
	saveState(legacyStateDir, "node_id", "5")
	saveState(legacyStateDir, "fleet_role", "active")

	target := resolveCommandTarget([]string{"--node", "5"})

	if target.stateDir != legacyStateDir {
		t.Fatalf("expected assigned node state dir %q, got %q", legacyStateDir, target.stateDir)
	}
}

func TestRandomFleetIDUsesTwelveLowercaseBase36Characters(t *testing.T) {
	fleetID := randomFleetID()

	if len(fleetID) != 12 {
		t.Fatalf("expected 12 character fleet id, got %q", fleetID)
	}
	for _, character := range fleetID {
		if character >= 'a' && character <= 'z' {
			continue
		}
		if character >= '0' && character <= '9' {
			continue
		}
		t.Fatalf("expected lowercase base36 fleet id, got %q", fleetID)
	}
}

func TestResolveVerifyTargetIgnoresTopLevelLabState(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	baseStateDir := filepath.Join(homeDirectory, ".internkim")
	if errorValue := os.MkdirAll(baseStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected base state dir: %v", errorValue)
	}
	saveState(baseStateDir, "device_url", "https://lab.example.test")
	saveState(baseStateDir, "board_ip", "192.0.2.40")

	target, errorValue := resolveVerifyTarget([]string{"--host", "192.0.2.10"})
	if errorValue != nil {
		t.Fatalf("expected verify target: %v", errorValue)
	}

	expectedStateDir := filepath.Join(baseStateDir, "devices", setup.BoardJetsonOrinNano)
	if target.stateDir != expectedStateDir {
		t.Fatalf("expected Jetson state dir %q, got %q", expectedStateDir, target.stateDir)
	}
	if target.host != "192.0.2.10" {
		t.Fatalf("expected explicit host, got %q", target.host)
	}
	if target.user != jetsonDefaultUser {
		t.Fatalf("expected Jetson user %q, got %q", jetsonDefaultUser, target.user)
	}
}

func TestResolveVerifyTargetUsesExplicitLabState(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	target, errorValue := resolveVerifyTarget([]string{"--board", "lab", "--host", "192.0.2.20"})
	if errorValue != nil {
		t.Fatalf("expected verify target: %v", errorValue)
	}

	expectedStateDir := filepath.Join(homeDirectory, ".internkim", "devices", "lab")
	if target.stateDir != expectedStateDir {
		t.Fatalf("expected lab state dir %q, got %q", expectedStateDir, target.stateDir)
	}
	if target.user != boardUser {
		t.Fatalf("expected lab user %q, got %q", boardUser, target.user)
	}
}

func TestResolveVerifyTargetAcceptsNodeArgument(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	baseStateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)
	nodeStateDir := filepath.Join(baseStateDir, "boards", "node-a")
	if errorValue := os.MkdirAll(nodeStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected node state dir: %v", errorValue)
	}
	saveState(nodeStateDir, "node_id", "node-a")
	saveState(nodeStateDir, "fleet_role", "active")

	target, errorValue := resolveVerifyTarget([]string{"--node", "node-a", "--host", "192.0.2.30"})
	if errorValue != nil {
		t.Fatalf("expected verify target: %v", errorValue)
	}

	if target.stateDir != nodeStateDir {
		t.Fatalf("expected node state dir %q, got %q", nodeStateDir, target.stateDir)
	}
	if target.nodeID != "node-a" {
		t.Fatalf("expected node-a, got %q", target.nodeID)
	}
}
