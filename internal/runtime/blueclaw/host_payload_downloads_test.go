package blueclaw_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// Two lists have to agree about what the package carries itself: the dependency
// declaration, which decides what `internkim install` demands of the machine, and the
// pin table, which decides what `internkim release packages` puts in the package. Nothing
// compared them, and the first reader of both was a person typing `internkim install`
// on a box whose package shipped no bun and no uv. Both directions are checked: a
// declared program with no pin builds a package that refuses to run, and a pin for a
// program nobody declares puts a binary in the package that nothing asks for.
func TestThePinsCoverExactlyThePayloadProgramsDeclared(t *testing.T) {
	declared := map[string]bool{}
	for _, programName := range blueclaw.HostProgramsThatArriveAsPayload() {
		declared[programName] = true
	}
	if len(declared) == 0 {
		t.Fatal("no dependency declares ArrivesAsPayload, so this test reads the wrong list")
	}

	for _, architecture := range debianArchitecturesThePinsCover(t) {
		downloads, errorValue := blueclaw.HostPayloadDownloads(architecture)
		if errorValue != nil {
			t.Fatalf("%s: %v", architecture, errorValue)
		}
		pinned := map[string]bool{}
		for _, download := range downloads {
			if !declared[download.ProgramName] {
				t.Errorf("%s pins %s, which no dependency declares ArrivesAsPayload; the package "+
					"would carry a binary nothing asks for", architecture, download.ProgramName)
			}
			pinned[download.ProgramName] = true
		}
		for programName := range declared {
			if !pinned[programName] {
				t.Errorf("%s is declared ArrivesAsPayload and no %s download is pinned for it, so "+
					"`internkim release packages` builds a package `internkim install` refuses at "+
					"\"1/5 Checking what this computer already has\"", programName, architecture)
			}
		}
	}
}

// Every vendored binary is fetched over TLS against a checksum. One that is not is how
// the media store becomes whatever answered the request.
func TestEveryPinnedPayloadIsFetchedOverHTTPSWithAChecksum(t *testing.T) {
	for _, architecture := range debianArchitecturesThePinsCover(t) {
		downloads, errorValue := blueclaw.HostPayloadDownloads(architecture)
		if errorValue != nil {
			t.Fatalf("%s: %v", architecture, errorValue)
		}
		for _, download := range downloads {
			if !strings.HasPrefix(download.URL, "https://") {
				t.Fatalf("%s for %s is fetched from %s", download.ProgramName, architecture, download.URL)
			}
			if len(download.SHA256) != 64 {
				t.Fatalf("%s for %s carries no sha256", download.ProgramName, architecture)
			}
			if !strings.Contains(download.URL, download.Version) {
				t.Fatalf("%s for %s is pinned at %s and fetched from %s",
					download.ProgramName, architecture, download.Version, download.URL)
			}
		}
	}
	if _, errorValue := blueclaw.HostPayloadDownloads("riscv64"); errorValue == nil {
		t.Fatal("an architecture with no pinned binaries was accepted, so a build for it would ship none of them")
	}
}

// The media store is one of the pinned downloads, not a second list of them.
func TestTheMediaReleaseIsDerivedFromTheOnePinTable(t *testing.T) {
	for _, architecture := range debianArchitecturesThePinsCover(t) {
		downloads, errorValue := blueclaw.HostPayloadDownloads(architecture)
		if errorValue != nil {
			t.Fatalf("%s: %v", architecture, errorValue)
		}
		found := false
		for _, download := range downloads {
			if download.ProgramName != blueclaw.BuzzMediaProgramName {
				continue
			}
			found = true
			machine := machineFor(t, architecture)
			release, isPublished := blueclaw.BuzzMediaReleaseFor(machine)
			if !isPublished {
				t.Fatalf("%s pins %s and BuzzMediaReleaseFor(%q) resolves to nothing",
					architecture, blueclaw.BuzzMediaProgramName, machine)
			}
			if release.SHA256 != download.SHA256 || release.URL != download.URL {
				t.Fatalf("the media release for %s is %s at %s and the pin table says %s at %s; "+
					"they are two lists again", machine, release.SHA256, release.URL, download.SHA256, download.URL)
			}
		}
		if !found {
			t.Fatalf("%s pins no %s, so a box for it would store attachments nowhere",
				architecture, blueclaw.BuzzMediaProgramName)
		}
	}
}

func debianArchitecturesThePinsCover(t *testing.T) []string {
	t.Helper()
	return []string{"arm64", "amd64"}
}

func machineFor(t *testing.T, debianArchitecture string) string {
	t.Helper()
	switch debianArchitecture {
	case "arm64":
		return "aarch64"
	case "amd64":
		return "x86_64"
	}
	t.Fatalf("no machine name for %s", debianArchitecture)
	return ""
}

// Every place that unpacks a vendored tarball has to ask for the path the declaration
// names. versity nests its binary under a directory named for the release, so the
// obvious `tar -xzf … versitygw` exits 2 with "Not found in archive" and installs
// nothing.
func TestTheDeviceExtractsTheMediaStoreFromThePathTheDeclarationNames(t *testing.T) {
	step, readError := os.ReadFile(filepath.Join(repositoryRootFromHere,
		"internal", "provisioning", "steps", "step_buzz_media.go"))
	if readError != nil {
		t.Fatal(readError)
	}
	if !strings.Contains(string(step), "release.PathInsideArchive") {
		t.Fatal("step_buzz_media.go unpacks the media tarball without asking the declaration where the " +
			"program sits, so a release that nests it differently installs nothing and reports success")
	}
}
