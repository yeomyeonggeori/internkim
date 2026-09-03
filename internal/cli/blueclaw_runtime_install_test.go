package cli

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestBlueclawRuntimeInstallPlanSkipsCurrentArtifacts(t *testing.T) {
	localManifest := blueclawRuntimeInstallManifestFixture("rootfs-sha")
	plan := buildBlueclawRuntimeInstallPlan(localManifest, blueclawRuntimeInstallManifestDocumentFixture("rootfs-sha"), map[string]bool{
		"cloud-hypervisor": true,
		"virtiofsd":        true,
		"vmlinux.bin":      true,
		"rootfs.ext4":      true,
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
		"cloud-hypervisor": true,
		"virtiofsd":        true,
		"vmlinux.bin":      true,
		"rootfs.ext4":      true,
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
		"cloud-hypervisor": true,
		"virtiofsd":        true,
		"vmlinux.bin":      true,
		"rootfs.ext4":      true,
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
		"cloud-hypervisor": false,
		"virtiofsd":        true,
		"vmlinux.bin":      true,
		"rootfs.ext4":      true,
	}, true, true)

	for _, artifact := range plan.artifacts {
		if artifact.name == "cloud-hypervisor" && !artifact.shouldInstall {
			t.Fatal("expected missing cloud-hypervisor to install")
		}
		if artifact.name == "cloud-hypervisor" && artifact.reason != "remote file missing" {
			t.Fatalf("expected missing file reason, got %q", artifact.reason)
		}
	}
}

func TestParseBlueclawRuntimeFilePresence(t *testing.T) {
	presence := parseBlueclawRuntimeFilePresence("cloud-hypervisor=present\nrootfs.ext4=missing\n")
	if !presence["cloud-hypervisor"] {
		t.Fatal("expected cloud-hypervisor present")
	}
	if presence["rootfs.ext4"] {
		t.Fatal("expected rootfs missing")
	}
}

func TestBlueclawRuntimeInstallCommandReplacesRootfsAtomically(t *testing.T) {
	artifact := blueclawRuntimeInstallArtifact{name: "rootfs.ext4", remotePath: "/opt/runtime/rootfs.ext4", mode: "0644"}
	command := blueclawRuntimeInstallCommand(artifact, "/tmp/rootfs.ext4")
	for _, expectedValue := range []string{
		"/opt/runtime/rootfs.ext4.next",
		"cp --sparse=always",
		"mv -f '/opt/runtime/rootfs.ext4.next' '/opt/runtime/rootfs.ext4'",
	} {
		if !strings.Contains(command, expectedValue) {
			t.Fatalf("expected rootfs install command to contain %q, got %s", expectedValue, command)
		}
	}
}

func blueclawRuntimeInstallManifestFixture(rootfsSHA256 string) blueclaw.RuntimeArtifactManifest {
	return blueclaw.RuntimeArtifactManifest{
		RuntimeName: "internkim-blueclaw-runtime",
		Platform:    "linux-arm64",
		Version:     "test",
		Files: []blueclaw.RuntimeArtifactManifestFile{
			{Name: "cloud-hypervisor", Path: "cloud-hypervisor", SHA256: "cloud-hypervisor-sha", Mode: "0755"},
			{Name: "virtiofsd", Path: "virtiofsd", SHA256: "virtiofsd-sha", Mode: "0755"},
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
    {"name": "cloud-hypervisor", "path": "cloud-hypervisor", "sha256": "cloud-hypervisor-sha", "mode": "0755"},
    {"name": "virtiofsd", "path": "virtiofsd", "sha256": "virtiofsd-sha", "mode": "0755"},
    {"name": "vmlinux.bin", "path": "vmlinux.bin", "sha256": "kernel-sha", "mode": "0644"},
    {"name": "rootfs.ext4", "path": "rootfs.ext4", "sha256": "` + rootfsSHA256 + `", "mode": "0644"}
  ]
}`
}
