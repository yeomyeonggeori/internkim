package machost

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAMonitorWithoutTheVirtualizationEntitlementIsRefused(t *testing.T) {
	executablePath := filepath.Join(t.TempDir(), "vfkit")
	if errorValue := os.WriteFile(executablePath, []byte("binary"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}

	errorValue := RequireVirtualMachineMonitor(executablePath, func(string) (string, error) {
		return "<key>com.apple.security.network.client</key>", nil
	})

	if errorValue == nil || !strings.Contains(errorValue.Error(), virtualizationEntitlement) {
		t.Fatalf("Virtualization.framework refuses an unentitled binary, and saying so here beats a boot that fails: %v", errorValue)
	}
}

func TestAnEntitledMonitorIsAccepted(t *testing.T) {
	executablePath := filepath.Join(t.TempDir(), "vfkit")
	if errorValue := os.WriteFile(executablePath, []byte("binary"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}

	errorValue := RequireVirtualMachineMonitor(executablePath, func(string) (string, error) {
		return "<key>" + virtualizationEntitlement + "</key><true/>", nil
	})

	if errorValue != nil {
		t.Fatalf("expected an entitled monitor to be accepted: %v", errorValue)
	}
}

func TestAMissingMonitorSaysHowToGetOne(t *testing.T) {
	errorValue := RequireVirtualMachineMonitor(filepath.Join(t.TempDir(), "absent"), func(string) (string, error) {
		return "", errors.New("codesign should not run on a file that is not there")
	})

	if errorValue == nil || !strings.Contains(errorValue.Error(), "brew install vfkit") {
		t.Fatalf("a missing monitor is the first thing a new host hits, so the message has to carry the fix: %v", errorValue)
	}
}

func TestOnlyAStatusOfOkCountsAsHealthy(t *testing.T) {
	for document, expected := range map[string]bool{
		`{"status":"ok","database":{"reachable":true}}`: true,
		`{"status":"unhealthy","database":{}}`:          false,
		`{"status":"ok"`:                                false,
		`not json at all`:                               false,
		``:                                              false,
	} {
		if guestHealthIsOK(document) != expected {
			t.Fatalf("a guest that boots but refuses work still answers, so only status ok counts: %q", document)
		}
	}
}

func TestAStartThatDiesAtOnceIsNotAStart(t *testing.T) {
	layout := NewLayout(t.TempDir(), "/tmp/bc")
	if errorValue := os.MkdirAll(layout.LogDirectoryPath(), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.MkdirAll(filepath.Dir(layout.SupervisorBinaryPath()), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(layout.SupervisorBinaryPath(), []byte("#!/bin/sh\necho cannot bind >&2\nexit 1\n"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}

	errorValue := StartSupervisor(layout)

	if errorValue == nil {
		t.Fatal("a supervisor reported as started is believed, and then whatever else listens on the address answers for the guest")
	}
	if !strings.Contains(errorValue.Error(), "cannot bind") {
		t.Fatalf("the reason has to travel with the failure, got %v", errorValue)
	}
}
