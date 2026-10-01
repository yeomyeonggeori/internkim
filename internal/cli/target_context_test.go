package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/deployops"
	setup "github.com/yeomyeonggeori/internkim/internal/provisioning/steps"
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

func TestTheVaultNamesThePhysicalDevice(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	stateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)
	if errorValue := os.MkdirAll(stateDir, 0o700); errorValue != nil {
		t.Fatalf("expected state dir: %v", errorValue)
	}
	saveState(stateDir, "device_url", "https://left-behind.example.test")
	saveState(stateDir, "fleet_secret", "left-behind-secret")
	t.Setenv(deployops.DeviceURLVariable, "https://fleetexample.example.test")
	t.Setenv(deployops.SSHHostnameVariable, "0.ssh.fleetexample.example.test")
	t.Setenv(deployops.FleetIDVariable, "fleetexample")
	t.Setenv(deployops.FleetSecretVariable, "vault-secret")

	target := resolveCommandTarget(nil)

	if target.deviceURL != "https://fleetexample.example.test" || target.sshHostname != "0.ssh.fleetexample.example.test" {
		t.Fatalf("the target was not the device the vault names: %+v", target)
	}
	if fleetID, fleetSecret, errorValue := target.fleetIdentity(); errorValue != nil || fleetID != "fleetexample" || fleetSecret != "vault-secret" {
		t.Fatalf("the fleet identity was not the vault's: %q %q %v", fleetID, fleetSecret, errorValue)
	}
}

func TestAVaultNamingNoDeviceLeavesThePhysicalTargetUnnamed(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	stateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)
	if errorValue := os.MkdirAll(stateDir, 0o700); errorValue != nil {
		t.Fatalf("expected state dir: %v", errorValue)
	}
	saveState(stateDir, "fleet_id", "left-behind")
	saveState(stateDir, "fleet_secret", "left-behind-secret")
	for _, name := range []string{deployops.DeviceURLVariable, deployops.SSHHostnameVariable, deployops.FleetIDVariable, deployops.FleetSecretVariable} {
		t.Setenv(name, "")
	}

	_, _, errorValue := resolveCommandTarget(nil).fleetIdentity()

	if errorValue == nil || !strings.Contains(errorValue.Error(), deployops.FleetSecretVariable) {
		t.Fatalf("a device state file stood in for the vault: %v", errorValue)
	}
}

func TestResolveCommandTargetReadsRemoteSSHFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(deployops.SSHHostnameVariable, "ssh.device.example.test")

	target := resolveCommandTarget([]string{"--remote-ssh"})

	if !target.useRemoteSSH {
		t.Fatalf("expected the remote ssh flag")
	}
	if target.sshHostname != "ssh.device.example.test" {
		t.Fatalf("expected the SSH hostname the vault names, got %q", target.sshHostname)
	}
}

func TestSetupCanRunWithoutSSH(t *testing.T) {
	if !setupCanRunWithoutSSH([]string{"--only", "blueclaw-payload-direct", "--force"}) {
		t.Fatal("expected blueclaw-payload-direct-only setup to allow public self-update without SSH")
	}
	if setupCanRunWithoutSSH([]string{"--only", "blueclaw-payload-direct,admind"}) {
		t.Fatal("expected mixed setup slice to require normal target resolution")
	}
	if setupCanRunWithoutSSH([]string{"--from", "blueclaw-payload-direct"}) {
		t.Fatal("expected --from setup to require normal target resolution")
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

	target := resolveCommandTarget([]string{"--node", "2"})
	expectedStateDir := filepath.Join(baseStateDir, "boards", "2")

	if target.stateDir != expectedStateDir {
		t.Fatalf("expected board scoped state dir %q, got %q", expectedStateDir, target.stateDir)
	}
	if loadState(target.stateDir, "node_id") != "2" {
		t.Fatalf("expected node id to be stored")
	}
	if loadState(target.stateDir, "fleet_id") != "fleet-one" {
		t.Fatalf("expected fleet id to be copied")
	}
	if loadState(target.stateDir, "fleet_secret") != "fleet-secret" {
		t.Fatalf("expected fleet secret to be copied")
	}
	if target.nodeID != "2" {
		t.Fatalf("expected node id, got %q", target.nodeID)
	}
}

func TestResolveCommandTargetAcceptsExplicitFleetJoin(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	target := resolveCommandTarget([]string{
		"--node",
		"3",
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

func TestResolveCommandTargetAcceptsOnlyPositiveNodeNumbers(t *testing.T) {
	validNodeIDs := []string{"1", "2", "10"}
	for _, nodeID := range validNodeIDs {
		if !isCommandTargetNodeNumber(nodeID) {
			t.Fatalf("expected node id %q to be valid", nodeID)
		}
	}

	invalidNodeIDs := []string{"", "0", "01", "node-a", "office-1", "Device 1"}
	for _, nodeID := range invalidNodeIDs {
		if isCommandTargetNodeNumber(nodeID) {
			t.Fatalf("expected node id %q to be invalid", nodeID)
		}
	}
}

func TestResolveCommandTargetStoresNumericNodeID(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	target := resolveCommandTarget([]string{"--node", "4"})

	if loadState(target.stateDir, "node_id") != "4" {
		t.Fatalf("expected DNS-safe node id, got %q", loadState(target.stateDir, "node_id"))
	}
}

func TestResolveCommandTargetDefaultsToSavedDefaultNode(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	baseStateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)
	nodeStateDir := filepath.Join(baseStateDir, "boards", "1")
	if errorValue := os.MkdirAll(nodeStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected node state dir: %v", errorValue)
	}
	saveState(baseStateDir, "default_node_id", "1")
	saveState(baseStateDir, "fleet_id", "fleet-one")
	saveState(nodeStateDir, "node_id", "1")
	saveState(nodeStateDir, "fleet_role", "active")

	target := resolveCommandTarget(nil)

	if target.stateDir != nodeStateDir {
		t.Fatalf("expected default node state dir %q, got %q", nodeStateDir, target.stateDir)
	}
	if target.nodeID != "1" {
		t.Fatalf("expected 1, got %q", target.nodeID)
	}
}

func TestResolveCommandTargetDefaultsToFirstActiveNode(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	baseStateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)
	activeStateDir := filepath.Join(baseStateDir, "boards", "1")
	pendingStateDir := filepath.Join(baseStateDir, "boards", "2")
	if errorValue := os.MkdirAll(activeStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected active node state dir: %v", errorValue)
	}
	if errorValue := os.MkdirAll(pendingStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected pending node state dir: %v", errorValue)
	}
	saveState(activeStateDir, "node_id", "1")
	saveState(activeStateDir, "fleet_role", "active")
	saveState(pendingStateDir, "node_id", "2")
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
	nodeStateDir := filepath.Join(baseStateDir, "boards", "1")
	if errorValue := os.MkdirAll(nodeStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected node state dir: %v", errorValue)
	}
	saveState(baseStateDir, "default_node_id", "1")
	saveState(nodeStateDir, "node_id", "1")
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
	nodeStateDir := filepath.Join(baseStateDir, "boards", "5")
	if errorValue := os.MkdirAll(nodeStateDir, 0o700); errorValue != nil {
		t.Fatalf("expected node state dir: %v", errorValue)
	}
	saveState(nodeStateDir, "node_id", "5")
	saveState(nodeStateDir, "fleet_role", "active")

	target, errorValue := resolveVerifyTarget([]string{"--node", "5", "--host", "192.0.2.30"})
	if errorValue != nil {
		t.Fatalf("expected verify target: %v", errorValue)
	}

	if target.stateDir != nodeStateDir {
		t.Fatalf("expected node state dir %q, got %q", nodeStateDir, target.stateDir)
	}
	if target.nodeID != "5" {
		t.Fatalf("expected 5, got %q", target.nodeID)
	}
}

func TestResolveVerifyTargetReachesAnExplicitHostByName(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	target, errorValue := resolveVerifyTarget([]string{
		"--remote-ssh",
		"--host", "ssh.example.test",
		"--user", "internkim",
		"--password", "blueclaw",
	})
	if errorValue != nil {
		t.Fatalf("expected verify target: %v", errorValue)
	}

	if target.sshClient == nil {
		t.Fatal("expected ssh client")
	}
	if target.sshClient.host != "ssh.example.test" {
		t.Fatalf("expected the explicit host to be reached by name, got %q", target.sshClient.host)
	}
}

func TestResolveVerifyTargetUsesTheSavedHostnameWithoutLocalProbe(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	stateDir := filepath.Join(homeDirectory, ".internkim", "devices", setup.BoardJetsonOrinNano)
	if errorValue := os.MkdirAll(stateDir, 0o700); errorValue != nil {
		t.Fatalf("expected state dir: %v", errorValue)
	}
	t.Setenv(deployops.SSHHostnameVariable, "0.ssh.example.test")

	target, errorValue := resolveVerifyTarget([]string{"--remote-ssh"})
	if errorValue != nil {
		t.Fatalf("expected verify target: %v", errorValue)
	}

	if target.host != "0.ssh.example.test" {
		t.Fatalf("expected the saved ssh hostname, got %q", target.host)
	}
	if target.sshClient == nil {
		t.Fatal("expected ssh client")
	}
}
