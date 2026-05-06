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
