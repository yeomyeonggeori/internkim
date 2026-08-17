package machost

import (
	"strings"
	"testing"
)

func TestEveryPathComesFromTheRootItWasGiven(t *testing.T) {
	layout := NewLayout("/somewhere/install", "/tmp/bc")

	for name, path := range map[string]string{
		"kernel":    layout.KernelImagePath(),
		"rootfs":    layout.RootFilesystemImagePath(),
		"workspace": layout.WorkspaceImagePath(),
		"delivery":  layout.DeliveryPath(),
		"runtime":   layout.RuntimeConfigurationPath(),
		"policy":    layout.PolicyPath(),
		"logs":      layout.LogDirectoryPath(),
		"binary":    layout.SupervisorBinaryPath(),
	} {
		if !strings.HasPrefix(path, "/somewhere/install/") {
			t.Fatalf("a layout that reads the environment cannot be tested on another machine: %s is %q", name, path)
		}
	}
}

func TestTheGuestDeliveryTreeIsTheOneTheGuestExpects(t *testing.T) {
	layout := NewLayout("/somewhere/install", "/tmp/bc")

	for expected, path := range map[string]string{
		"/somewhere/install/delivery/config":              layout.DeliveryConfigurationPath(),
		"/somewhere/install/delivery/runtime/current":     layout.DeliveryRuntimePath(),
		"/somewhere/install/delivery/skills":              layout.DeliverySkillsPath(),
		"/somewhere/install/delivery/config/runtime.json": layout.RuntimeConfigurationPath(),
	} {
		if path != expected {
			t.Fatalf("the guest reads a fixed tree under /delivery, expected %q, got %q", expected, path)
		}
	}
}

func TestARuntimeDirectoryTooLongForASocketIsRefusedAtInstall(t *testing.T) {
	testCases := []struct {
		runtimeRootPath string
		isAccepted      bool
	}{
		{"/tmp/bc-501", true},
		{strings.Repeat("a", 35), true},
		{strings.Repeat("a", 36), false},
		{"/Users/someone/.internkim/blueclaw/runtime", false},
	}

	for _, testCase := range testCases {
		errorValue := NewLayout("/somewhere/install", testCase.runtimeRootPath).AssertVSockSocketPathFits()
		if testCase.isAccepted && errorValue != nil {
			t.Fatalf("expected %q to fit: %v", testCase.runtimeRootPath, errorValue)
		}
		if !testCase.isAccepted && errorValue == nil {
			t.Fatalf("vfkit reports the overflow as a malformed flag, so %q has to be caught here", testCase.runtimeRootPath)
		}
	}
}
