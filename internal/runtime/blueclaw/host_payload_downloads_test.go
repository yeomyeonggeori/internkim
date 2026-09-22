package blueclaw_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The image, the device's provisioning step and the .deb install the same
// vendored binaries from the same upstream releases, and they say so in two
// languages. This reads what host/Dockerfile actually pins and requires it to be
// what Go declares, in both directions, so a bump applied to one is not a company
// host whose halves run different binaries.
//
// It replaced two narrower tests: one that read only the media store's pin, and one
// that read only the browsers'. Three pinned downloads had grown two mechanisms.
// Two lists have to agree about what the package carries itself: the dependency
// declaration, which decides what `internkim install` demands of the machine, and the
// pin table, which decides what `internkim release deb` puts in the package. Nothing
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
					"`internkim release deb` builds a package `internkim install` refuses at "+
					"\"1/5 Checking what this computer already has\"", programName, architecture)
			}
		}
	}
}

func TestTheHostImagePinsWhatTheDeclarationPins(t *testing.T) {
	document, readError := os.ReadFile(filepath.Join(repositoryRootFromHere, "host", "Dockerfile"))
	if readError != nil {
		t.Fatal(readError)
	}
	dockerfile := string(document)

	declaredByVariable := map[string]map[string]bool{}
	versionByArgument := map[string]string{}
	for _, architecture := range debianArchitecturesThePinsCover(t) {
		downloads, errorValue := blueclaw.HostPayloadDownloads(architecture)
		if errorValue != nil {
			t.Fatalf("%s: %v", architecture, errorValue)
		}
		for _, download := range downloads {
			if declaredByVariable[download.DockerfileChecksumVariable] == nil {
				declaredByVariable[download.DockerfileChecksumVariable] = map[string]bool{}
			}
			declaredByVariable[download.DockerfileChecksumVariable][download.SHA256] = true
			versionByArgument[download.DockerfileVersionArgument] = download.Version
		}
	}
	if len(declaredByVariable) == 0 {
		t.Fatal("the declaration pins nothing, so this test reads the wrong place")
	}

	for argument, version := range versionByArgument {
		if !strings.Contains(dockerfile, "ARG "+argument+"="+version) {
			t.Fatalf("host/Dockerfile does not carry %q; internal/runtime/blueclaw declares that version",
				"ARG "+argument+"="+version)
		}
	}

	for variable, declared := range declaredByVariable {
		pinned := checksumsAssignedTo(dockerfile, variable+"=")
		if len(pinned) == 0 {
			t.Fatalf("host/Dockerfile assigns no %s, so it downloads that program unchecked", variable)
		}
		for _, checksum := range pinned {
			if !declared[checksum] {
				t.Fatalf("host/Dockerfile pins %s=%s, which internal/runtime/blueclaw declares for no "+
					"architecture; the image and the package would install different binaries", variable, checksum)
			}
			delete(declared, checksum)
		}
		if len(declared) != 0 {
			t.Fatalf("internal/runtime/blueclaw declares %s checksums the host image does not pin: %v",
				variable, declared)
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
// nothing; the image, the device's provisioning step and the package all extract it,
// and nothing but a shared declaration keeps the three in step.
func TestEveryExtractionAsksForThePathTheDeclarationNames(t *testing.T) {
	dockerfile, readError := os.ReadFile(filepath.Join(repositoryRootFromHere, "host", "Dockerfile"))
	if readError != nil {
		t.Fatal(readError)
	}
	step, readError := os.ReadFile(filepath.Join(repositoryRootFromHere,
		"internal", "provisioning", "steps", "step_buzz_media.go"))
	if readError != nil {
		t.Fatal(readError)
	}
	if !strings.Contains(string(step), "release.PathInsideArchive") {
		t.Fatal("step_buzz_media.go unpacks the media tarball without asking the declaration where the " +
			"program sits, so a release that nests it differently installs nothing and reports success")
	}

	downloads, errorValue := blueclaw.HostPayloadDownloads("arm64")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	checked := 0
	for _, download := range downloads {
		if download.PathInsideArchive == "" {
			continue
		}
		checked++
		directory, _, isNested := strings.Cut(download.PathInsideArchive, "/")
		if !isNested {
			continue
		}
		if !strings.Contains(string(dockerfile), directory) &&
			!strings.Contains(string(dockerfile), "${"+download.DockerfileVersionArgument+"}") {
			t.Fatalf("%s is nested under %s in its tarball and host/Dockerfile names neither that "+
				"directory nor %s, so its extraction asks for a member that is not there",
				download.ProgramName, directory, download.DockerfileVersionArgument)
		}
	}
	if checked == 0 {
		t.Fatal("no archived download was checked, so this test reads the wrong place")
	}
}
