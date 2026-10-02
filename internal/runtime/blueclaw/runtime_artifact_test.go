package blueclaw

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateRuntimeArtifactDirectoryRequiresManifestAndChecksums(t *testing.T) {
	artifactDirectoryPath := t.TempDir()
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "cloud-hypervisor", "cloud-hypervisor")
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "virtiofsd", "virtiofsd")
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "vmlinux.bin", "kernel")
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "rootfs.ext4", "rootfs")

	manifestDocument := `{
  "runtimeName": "internkim-blueclaw-runtime",
  "platform": "linux-arm64",
  "version": "test",
  "files": [
    {"name": "cloud-hypervisor", "path": "cloud-hypervisor", "sha256": "` + runtimeArtifactTestSHA256("cloud-hypervisor") + `", "mode": "0755"},
    {"name": "virtiofsd", "path": "virtiofsd", "sha256": "` + runtimeArtifactTestSHA256("virtiofsd") + `", "mode": "0755"},
    {"name": "vmlinux.bin", "path": "vmlinux.bin", "sha256": "` + runtimeArtifactTestSHA256("kernel") + `", "mode": "0644"},
    {"name": "rootfs.ext4", "path": "rootfs.ext4", "sha256": "` + runtimeArtifactTestSHA256("rootfs") + `", "mode": "0644"}
  ]
}
`
	if errorValue := os.WriteFile(filepath.Join(artifactDirectoryPath, "manifest.json"), []byte(manifestDocument), 0o644); errorValue != nil {
		t.Fatalf("expected manifest to be written: %v", errorValue)
	}

	manifest, errorValue := ValidateRuntimeArtifactDirectory(artifactDirectoryPath)
	if errorValue != nil {
		t.Fatalf("expected artifact directory to validate: %v", errorValue)
	}
	if manifest.Platform != "linux-arm64" {
		t.Fatalf("expected platform to match, got %q", manifest.Platform)
	}
}

func TestValidateRuntimeArtifactDirectoryRejectsChecksumMismatch(t *testing.T) {
	artifactDirectoryPath := t.TempDir()
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "cloud-hypervisor", "cloud-hypervisor")
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "virtiofsd", "virtiofsd")
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "vmlinux.bin", "kernel")
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "rootfs.ext4", "rootfs")

	manifestDocument := `{
  "runtimeName": "internkim-blueclaw-runtime",
  "platform": "linux-arm64",
  "version": "test",
  "files": [
    {"name": "cloud-hypervisor", "path": "cloud-hypervisor", "sha256": "bad", "mode": "0755"},
    {"name": "virtiofsd", "path": "virtiofsd", "sha256": "` + runtimeArtifactTestSHA256("virtiofsd") + `", "mode": "0755"},
    {"name": "vmlinux.bin", "path": "vmlinux.bin", "sha256": "` + runtimeArtifactTestSHA256("kernel") + `", "mode": "0644"},
    {"name": "rootfs.ext4", "path": "rootfs.ext4", "sha256": "` + runtimeArtifactTestSHA256("rootfs") + `", "mode": "0644"}
  ]
}
`
	if errorValue := os.WriteFile(filepath.Join(artifactDirectoryPath, "manifest.json"), []byte(manifestDocument), 0o644); errorValue != nil {
		t.Fatalf("expected manifest to be written: %v", errorValue)
	}

	_, errorValue := ValidateRuntimeArtifactDirectory(artifactDirectoryPath)
	if errorValue == nil {
		t.Fatal("expected checksum mismatch to fail")
	}
}

func TestValidateRuntimeArtifactSourceAcceptsBaseMetadataWithoutBlueclawRevision(t *testing.T) {
	repositoryRootPath := runtimeArtifactRepositoryRoot(t)
	expectedManifest, errorValue := ExpectedRuntimeArtifactSource(repositoryRootPath)
	if errorValue != nil {
		t.Fatalf("expected source metadata: %v", errorValue)
	}
	manifest := RuntimeArtifactManifest{
		RuntimeName:         "internkim-blueclaw-runtime",
		Platform:            "linux-arm64",
		Version:             "test",
		GuestInitSHA256:     expectedManifest.GuestInitSHA256,
		PrepareScriptSHA256: expectedManifest.PrepareScriptSHA256,
		BaseSourceSHA256:    expectedManifest.BaseSourceSHA256,
	}
	if errorValue := ValidateRuntimeArtifactSource(repositoryRootPath, manifest); errorValue != nil {
		t.Fatalf("expected source metadata to validate: %v", errorValue)
	}
}

func TestPrepareRuntimeScriptReusesExistingArtifactKernel(t *testing.T) {
	repositoryRootPath := runtimeArtifactRepositoryRoot(t)
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-runtime"))
	if errorValue != nil {
		t.Fatalf("expected prepare script: %v", errorValue)
	}
	script := string(document)
	expectedFragments := []string{
		`if published_kernel_matches_configuration; then`,
		`install -m 0644 "$artifact_directory/vmlinux.bin" "$stage_directory/vmlinux.bin"`,
		`kernel_cache_path="$cache_directory/vmlinux-$(guest_kernel_build_identifier)-aarch64.bin"`,
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected prepare script to contain %q", fragment)
		}
	}
	if strings.Index(script, expectedFragments[0]) > strings.Index(script, expectedFragments[2]) {
		t.Fatalf("expected artifact kernel reuse before cache fallback")
	}
}

func TestPrepareRuntimeScriptKeysTheGuestKernelToItsConfiguration(t *testing.T) {
	repositoryRootPath := runtimeArtifactRepositoryRoot(t)
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-runtime"))
	if errorValue != nil {
		t.Fatalf("expected prepare script: %v", errorValue)
	}
	script := string(document)
	expectedFragments := []string{
		`guest_kernel_configuration_path="$repository_root/assets/blueclaw-runtime/guest-kernel-aarch64.config"`,
		`"guestKernelConfigurationSHA256": guest_kernel_configuration_sha256,`,
		`manifest.get("guestKernelConfigurationSHA256") == configuration_sha`,
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected prepare script to contain %q", fragment)
		}
	}
	if strings.Contains(script, "microvm-kernel-ci-aarch64") {
		t.Fatal("expected the guest kernel configuration to be carried here, not borrowed from another monitor")
	}
}

func TestGuestKernelConfigurationCarriesVirtioOverPCIOnly(t *testing.T) {
	repositoryRootPath := runtimeArtifactRepositoryRoot(t)
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "assets", "blueclaw-runtime", "guest-kernel-aarch64.config"))
	if errorValue != nil {
		t.Fatalf("expected guest kernel configuration: %v", errorValue)
	}
	configuration := string(document)
	if !strings.Contains(configuration, "\n# CONFIG_VIRTIO_MMIO is not set\n") {
		t.Fatal("only Firecracker reached virtio over MMIO, so the guest kernel must not carry it")
	}
	for _, symbol := range []string{
		"CONFIG_SERIAL_8250_CONSOLE=y",
		"CONFIG_PCI=y",
		"CONFIG_PCI_HOST_GENERIC=y",
		"CONFIG_VIRTIO_PCI=y",
		"CONFIG_SERIAL_AMBA_PL011_CONSOLE=y",
		"CONFIG_FUSE_FS=y",
		"CONFIG_VIRTIO_FS=y",
	} {
		if !strings.Contains(configuration, "\n"+symbol+"\n") {
			t.Fatalf("expected guest kernel configuration to set %s", symbol)
		}
	}
}

func TestPrepareRuntimeScriptStopsOnlySelfStartedContainerBuilder(t *testing.T) {
	repositoryRootPath := runtimeArtifactRepositoryRoot(t)
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-runtime"))
	if errorValue != nil {
		t.Fatalf("expected prepare script: %v", errorValue)
	}
	script := string(document)
	expectedFragments := []string{
		`INTERNKIM_KEEP_CONTAINER_VM`,
		`container_status_before="$("$repository_root/internkim" lab status 2>/dev/null || true)"`,
		`if [ "$container_status_before" != "running" ]`,
		`trap cleanup_container_builder EXIT`,
		`"$repository_root/internkim" lab vm-down`,
		`trap - EXIT`,
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected prepare script to contain %q", fragment)
		}
	}
}

func TestPrepareRuntimeScriptInstallsBlueclawGuestCalculator(t *testing.T) {
	repositoryRootPath := runtimeArtifactRepositoryRoot(t)
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-runtime"))
	if errorValue != nil {
		t.Fatalf("expected prepare script: %v", errorValue)
	}
	script := string(document)
	expectedFragments := []string{
		`rootfs_base_packages="ca-certificates,curl,bash,bc,coreutils`,
		`command -v bc >/dev/null`,
		`UV_UNMANAGED_INSTALL=/usr/local/bin`,
		`install -m 0755 "$work_directory/bin/blueclaw-posix-helper"`,
		`chown 0:0 "$rootfs_directory/usr/local/bin/blueclaw-posix-helper"`,
		`chmod 4755 "$rootfs_directory/usr/local/bin/blueclaw-posix-helper"`,
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected prepare script to contain %q", fragment)
		}
	}
}

func writeRuntimeArtifactFile(t *testing.T, artifactDirectoryPath string, fileName string, content string) {
	t.Helper()
	if errorValue := os.WriteFile(filepath.Join(artifactDirectoryPath, fileName), []byte(content), 0o644); errorValue != nil {
		t.Fatalf("expected artifact file to be written: %v", errorValue)
	}
}

func runtimeArtifactTestSHA256(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

func runtimeArtifactRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filePath, _, isOK := runtime.Caller(0)
	if !isOK {
		t.Fatal("expected caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filePath), "..", "..", ".."))
}
