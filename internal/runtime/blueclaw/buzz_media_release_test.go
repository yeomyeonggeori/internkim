package blueclaw_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestEveryPublishedMachineResolvesToAPinnedTarball(t *testing.T) {
	for _, machine := range blueclaw.BuzzMediaPublishedMachines() {
		release, isPublished := blueclaw.BuzzMediaReleaseFor(machine)
		if !isPublished {
			t.Fatalf("%s is listed as published and resolves to nothing", machine)
		}
		if len(release.SHA256) != 64 {
			t.Fatalf("%s resolves to checksum %q, which is not a sha256; an unchecked download is how "+
				"the media store becomes whatever answered the request", machine, release.SHA256)
		}
		if !strings.Contains(release.URL, "v"+blueclaw.BuzzMediaVersion+"/") ||
			!strings.Contains(release.AssetName, blueclaw.BuzzMediaVersion) {
			t.Fatalf("%s resolves to %s at %s, which does not name version %s",
				machine, release.AssetName, release.URL, blueclaw.BuzzMediaVersion)
		}
	}
	if _, isPublished := blueclaw.BuzzMediaReleaseFor("riscv64"); isPublished {
		t.Fatal("an architecture versity does not publish resolved to a release, so the provisioning step " +
			"would download a tarball that does not exist instead of saying which architectures exist")
	}
}

// The device path and the agent image install the same media store from the
// same upstream release, and they say so in two languages. This reads what the
// Dockerfile actually pins and requires it to be what Go declares, so a bump
// applied to one is not a company host whose two halves run different servers.
func TestTheHostImagePinsTheDeclaredMediaRelease(t *testing.T) {
	document, readError := os.ReadFile(filepath.Join(repositoryRootFromHere, "host", "Dockerfile"))
	if readError != nil {
		t.Fatal(readError)
	}
	dockerfile := string(document)

	declaredVersion := "ARG VERSITYGW_VERSION=" + blueclaw.BuzzMediaVersion
	if !strings.Contains(dockerfile, declaredVersion) {
		t.Fatalf("host/Dockerfile does not carry %q; internal/runtime/blueclaw declares versitygw %s",
			declaredVersion, blueclaw.BuzzMediaVersion)
	}

	pinned := checksumsAssignedTo(dockerfile, "versitygwSHA256=")
	if len(pinned) == 0 {
		t.Fatal("host/Dockerfile assigns no versitygwSHA256, so this test reads nothing and would pass " +
			"on an image that downloads the media store unchecked")
	}
	declared := map[string]bool{}
	for _, machine := range blueclaw.BuzzMediaPublishedMachines() {
		release, _ := blueclaw.BuzzMediaReleaseFor(machine)
		declared[release.SHA256] = true
	}
	for _, checksum := range pinned {
		if !declared[checksum] {
			t.Fatalf("host/Dockerfile pins versitygw checksum %s, which internal/runtime/blueclaw does not "+
				"declare for any architecture; the image and the device path would install different binaries",
				checksum)
		}
		delete(declared, checksum)
	}
	if len(declared) != 0 {
		t.Fatalf("internal/runtime/blueclaw declares versitygw checksums the host image does not pin: %v", declared)
	}
}

func checksumsAssignedTo(document string, assignment string) []string {
	checksums := []string{}
	for _, logicalLine := range logicalLinesOf(document) {
		for _, field := range strings.Fields(logicalLine) {
			trimmed := strings.TrimSuffix(strings.TrimSuffix(field, ";"), ";;")
			value, isAssignment := strings.CutPrefix(trimmed, assignment)
			if isAssignment && value != "" {
				checksums = append(checksums, value)
			}
		}
	}
	return checksums
}
