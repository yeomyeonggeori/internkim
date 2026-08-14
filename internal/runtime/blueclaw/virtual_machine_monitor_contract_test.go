package blueclaw

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVirtualMachineMonitorNamesMatchBlueclaw(t *testing.T) {
	repositoryRootPath := runtimeArtifactRepositoryRoot(t)
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, BlueclawSubmodulePath, "internal", "firecracker", "virtual_machine_monitor.go"))
	if errorValue != nil {
		t.Fatalf("expected the blueclaw monitor source: %v", errorValue)
	}
	source := strings.Join(strings.Fields(string(document)), " ")

	for constantName, expectedValue := range map[string]string{
		"FirecrackerMonitorName":     FirecrackerMonitorName,
		"CloudHypervisorMonitorName": CloudHypervisorMonitorName,
	} {
		declaration := constantName + " = \"" + expectedValue + "\""
		if !strings.Contains(source, declaration) {
			t.Fatalf("blueclaw no longer declares %s; runtime.json would name a monitor it cannot select", declaration)
		}
	}
}

func TestServiceUnitConfinesTheFilesystemOnlyWithoutTheJailer(t *testing.T) {
	unitDocument := BlueclawServiceUnit()
	carriesConfinement := strings.Contains(unitDocument, "ProtectSystem=strict")

	if BlueclawVirtualMachineMonitor == FirecrackerMonitorName && carriesConfinement {
		t.Fatal("the jailer hard links the runtime assets into its chroot, which ProtectSystem= breaks with a cross-device link")
	}
	if BlueclawVirtualMachineMonitor == CloudHypervisorMonitorName && !carriesConfinement {
		t.Fatal("dropping the jailer without confining the unit loses the chroot with nothing in its place")
	}
	if !carriesConfinement {
		return
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
