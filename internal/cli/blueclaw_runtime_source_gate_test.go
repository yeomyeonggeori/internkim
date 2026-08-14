package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeInstallPlanReportsWhenTheGuestImageShips(t *testing.T) {
	planWithoutRootFilesystem := blueclawRuntimeInstallPlan{artifacts: []blueclawRuntimeInstallArtifact{
		{name: "vmlinux.bin", shouldInstall: true},
		{name: "cloud-hypervisor", shouldInstall: true},
		{name: "rootfs.ext4", shouldInstall: false},
	}}
	if blueclawRuntimeInstallPlanInstallsRootFilesystem(planWithoutRootFilesystem) {
		t.Fatal("a kernel-only install does not ship the guest image")
	}

	planWithRootFilesystem := blueclawRuntimeInstallPlan{artifacts: []blueclawRuntimeInstallArtifact{
		{name: "rootfs.ext4", shouldInstall: true},
	}}
	if !blueclawRuntimeInstallPlanInstallsRootFilesystem(planWithRootFilesystem) {
		t.Fatal("expected the guest image install to be reported")
	}
}

func TestPrepareRuntimeScriptStatesWhereASuppliedGuestImageCameFrom(t *testing.T) {
	repositoryRootPath := runtimeSourceGateRepositoryRoot(t)
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-runtime"))
	if errorValue != nil {
		t.Fatalf("expected prepare script: %v", errorValue)
	}
	script := string(document)

	for _, fragment := range []string{
		`guest_init_sha="$(supplied_rootfs_provenance guestInitSHA256)"`,
		`prepare_script_sha="$(supplied_rootfs_provenance prepareScriptSHA256)"`,
		`base_source_sha="$(supplied_rootfs_provenance baseSourceSHA256)"`,
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("a supplied guest image must state its own provenance, missing %q", fragment)
		}
	}

	if !strings.Contains(script, "needs a manifest.json beside it") {
		t.Fatal("a supplied guest image without provenance must be refused rather than given this checkout's")
	}
}

func runtimeSourceGateRepositoryRoot(t *testing.T) string {
	t.Helper()
	workingDirectory, errorValue := os.Getwd()
	if errorValue != nil {
		t.Fatalf("expected working directory: %v", errorValue)
	}
	for directory := workingDirectory; directory != "/"; directory = filepath.Dir(directory) {
		if _, errorValue := os.Stat(filepath.Join(directory, "go.mod")); errorValue == nil {
			return directory
		}
	}
	t.Fatal("expected to find the repository root")
	return ""
}
