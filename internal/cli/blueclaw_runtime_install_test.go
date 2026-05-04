package cli

import (
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestBlueclawRuntimeInstallPlanSkipsCurrentArtifacts(t *testing.T) {
	localManifest := blueclawRuntimeInstallManifestFixture("rootfs-sha")
	plan := buildBlueclawRuntimeInstallPlan(localManifest, blueclawRuntimeInstallManifestDocumentFixture("rootfs-sha"), map[string]bool{
		"firecracker": true,
		"jailer":      true,
		"vmlinux.bin": true,
		"rootfs.ext4": true,
	}, true, true)

	if plan.shouldInstallManifest {
		t.Fatal("expected current manifest to be skipped")
	}
	for _, artifact := range plan.artifacts {
		if artifact.shouldInstall {
			t.Fatalf("expected %s to be skipped", artifact.name)
		}
		if artifact.reason != "" {
			t.Fatalf("expected %s reason to be empty, got %q", artifact.name, artifact.reason)
		}
	}
}

func TestBlueclawRuntimeInstallPlanReinstallsBrokenRootfs(t *testing.T) {
	localManifest := blueclawRuntimeInstallManifestFixture("rootfs-sha")
	plan := buildBlueclawRuntimeInstallPlan(localManifest, blueclawRuntimeInstallManifestDocumentFixture("rootfs-sha"), map[string]bool{
		"firecracker": true,
		"jailer":      true,
		"vmlinux.bin": true,
		"rootfs.ext4": true,
	}, false, true)

	if !plan.shouldInstallManifest {
		t.Fatal("expected manifest to be reinstalled with repaired rootfs")
	}
	for _, artifact := range plan.artifacts {
		if artifact.name == "rootfs.ext4" && !artifact.shouldInstall {
			t.Fatal("expected rootfs to reinstall when contract check fails")
		}
		if artifact.name == "rootfs.ext4" && artifact.reason != "rootfs base contract changed" {
			t.Fatalf("expected rootfs contract reason, got %q", artifact.reason)
		}
		if artifact.name != "rootfs.ext4" && artifact.shouldInstall {
			t.Fatalf("expected %s to be skipped", artifact.name)
		}
	}
}

func TestBlueclawRuntimeInstallPlanInstallsChangedArtifact(t *testing.T) {
	localManifest := blueclawRuntimeInstallManifestFixture("new-rootfs-sha")
	plan := buildBlueclawRuntimeInstallPlan(localManifest, blueclawRuntimeInstallManifestDocumentFixture("old-rootfs-sha"), map[string]bool{
		"firecracker": true,
		"jailer":      true,
		"vmlinux.bin": true,
		"rootfs.ext4": true,
	}, true, false)

	if !plan.shouldInstallManifest {
		t.Fatal("expected manifest to install when an artifact changes")
	}
	for _, artifact := range plan.artifacts {
		if artifact.name == "rootfs.ext4" && !artifact.shouldInstall {
			t.Fatal("expected changed rootfs to install")
		}
		if artifact.name == "rootfs.ext4" && artifact.reason != "checksum changed" {
			t.Fatalf("expected changed rootfs checksum reason, got %q", artifact.reason)
		}
	}
}

func TestBlueclawRuntimeInstallPlanRecordsMissingRemoteFile(t *testing.T) {
	localManifest := blueclawRuntimeInstallManifestFixture("rootfs-sha")
	plan := buildBlueclawRuntimeInstallPlan(localManifest, blueclawRuntimeInstallManifestDocumentFixture("rootfs-sha"), map[string]bool{
		"firecracker": false,
		"jailer":      true,
		"vmlinux.bin": true,
		"rootfs.ext4": true,
	}, true, true)

	for _, artifact := range plan.artifacts {
		if artifact.name == "firecracker" && !artifact.shouldInstall {
			t.Fatal("expected missing firecracker to install")
		}
		if artifact.name == "firecracker" && artifact.reason != "remote file missing" {
			t.Fatalf("expected missing file reason, got %q", artifact.reason)
		}
	}
}

func TestParseBlueclawRuntimeFilePresence(t *testing.T) {
	presence := parseBlueclawRuntimeFilePresence("firecracker=present\nrootfs.ext4=missing\n")
	if !presence["firecracker"] {
		t.Fatal("expected firecracker present")
	}
	if presence["rootfs.ext4"] {
		t.Fatal("expected rootfs missing")
	}
}

func blueclawRuntimeInstallManifestFixture(rootfsSHA256 string) blueclaw.RuntimeArtifactManifest {
	return blueclaw.RuntimeArtifactManifest{
		RuntimeName: "internkim-blueclaw-runtime",
		Platform:    "linux-arm64",
		Version:     "test",
		Files: []blueclaw.RuntimeArtifactManifestFile{
			{Name: "firecracker", Path: "firecracker", SHA256: "firecracker-sha", Mode: "0755"},
			{Name: "jailer", Path: "jailer", SHA256: "jailer-sha", Mode: "0755"},
			{Name: "vmlinux.bin", Path: "vmlinux.bin", SHA256: "kernel-sha", Mode: "0644"},
			{Name: "rootfs.ext4", Path: "rootfs.ext4", SHA256: rootfsSHA256, Mode: "0644"},
		},
	}
}

func blueclawRuntimeInstallManifestDocumentFixture(rootfsSHA256 string) string {
	return `{
  "runtimeName": "internkim-blueclaw-runtime",
  "platform": "linux-arm64",
  "version": "test",
  "files": [
    {"name": "firecracker", "path": "firecracker", "sha256": "firecracker-sha", "mode": "0755"},
    {"name": "jailer", "path": "jailer", "sha256": "jailer-sha", "mode": "0755"},
    {"name": "vmlinux.bin", "path": "vmlinux.bin", "sha256": "kernel-sha", "mode": "0644"},
    {"name": "rootfs.ext4", "path": "rootfs.ext4", "sha256": "` + rootfsSHA256 + `", "mode": "0644"}
  ]
}`
}
