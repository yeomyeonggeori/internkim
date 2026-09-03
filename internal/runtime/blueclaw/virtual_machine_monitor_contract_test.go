package blueclaw

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVirtualMachineMonitorNamesMatchBlueclaw(t *testing.T) {
	repositoryRootPath := runtimeArtifactRepositoryRoot(t)
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, BlueclawSubmodulePath, "internal", "guest", "virtual_machine_monitor.go"))
	if errorValue != nil {
		t.Fatalf("expected the blueclaw monitor source: %v", errorValue)
	}
	source := strings.Join(strings.Fields(string(document)), " ")

	for constantName, expectedValue := range map[string]string{
		"CloudHypervisorMonitorName": CloudHypervisorMonitorName,
	} {
		declaration := constantName + " = \"" + expectedValue + "\""
		if !strings.Contains(source, declaration) {
			t.Fatalf("blueclaw no longer declares %s; runtime.json would name a monitor it cannot select", declaration)
		}
	}
}

func TestServiceUnitConfinesTheFilesystem(t *testing.T) {
	unitDocument := BlueclawServiceUnit()
	if !strings.Contains(unitDocument, "ProtectSystem=strict") {
		t.Fatal("the monitor runs with no chroot of its own, so the unit must confine the filesystem")
	}
	for _, writablePath := range []string{
		BlueclawRuntimeInstanceDirectoryPath,
		filepath.Dir(BlueclawWorkspaceImagePath),
		BlueclawSupervisorLogDirectoryPath,
		BlueclawWorkspacePath,
	} {
		if !strings.Contains(unitDocument, writablePath) {
			t.Fatalf("expected %q to stay writable under confinement", writablePath)
		}
	}
}
