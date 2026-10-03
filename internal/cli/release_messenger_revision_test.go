package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const relayRevisionBeforeTheDeclaredMimePatch = "973386351646d01efca2735ad4921c0d219cff85" +
	"+rust-s3-786653e1b4abd47cb046ef1227aa646d97073c2c+traceless-delete-1+rustls-0.23.45"

const declaredMimePatchPath = "tools/buzz-relay-patches/declared-mime-agrees-with-stored.patch"

func repositoryWithThePrepareScript(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	for _, source := range []string{messengerPrepareScriptPath, declaredMimePatchPath} {
		content, errorValue := os.ReadFile(filepath.Join("..", "..", source))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		writeExecutableForTest(t, filepath.Join(repository, source), content)
	}
	return repository
}

func writeExecutableForTest(t *testing.T, path string, content []byte) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, content, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func recordMessengerRevision(t *testing.T, repository string, artifactDirectory string, revision string) {
	t.Helper()
	writeExecutableForTest(t, filepath.Join(repository, artifactDirectory, messengerRevisionFileName), []byte(revision+"\n"))
}

func linuxMessengerRepository(t *testing.T) string {
	t.Helper()
	repository := repositoryWithThePrepareScript(t)
	program, errorValue := os.ReadFile(compileLinuxProgram(t, "arm64"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, name := range messengerProgramNames {
		writeExecutableForTest(t, filepath.Join(repository, blueclaw.BuzzRelayArtifactPath, name), program)
	}
	return repository
}

func macMessengerRepository(t *testing.T) string {
	t.Helper()
	repository := repositoryWithThePrepareScript(t)
	for _, name := range messengerProgramNames {
		writeMachOForTest(t, filepath.Join(repository, brewMessengerArtifactPath, name), 13)
	}
	return repository
}

func revisionThisTreeBuilds(t *testing.T, repository string) string {
	t.Helper()
	revision, errorValue := expectedMessengerRevision(repository)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(revision, "+declared-mime-") {
		t.Fatalf("the prepare script printed %q, which names no declared-mime patch", revision)
	}
	return revision
}

func requireRefusalNamesBothRevisions(t *testing.T, errorValue error, expected string, found string, fix string) {
	t.Helper()
	if errorValue == nil {
		t.Fatal("a messenger built at a revision this tree no longer builds was packaged")
	}
	for _, fragment := range []string{blueclaw.BuzzRelayName, expected, found, fix} {
		if !strings.Contains(errorValue.Error(), fragment) {
			t.Errorf("the refusal does not name %q: %v", fragment, errorValue)
		}
	}
}

func TestALinuxPackageRefusesARelayBuiltBeforeTheTreesPatches(t *testing.T) {
	repository := linuxMessengerRepository(t)
	recordMessengerRevision(t, repository, blueclaw.BuzzRelayArtifactPath, relayRevisionBeforeTheDeclaredMimePatch)

	_, errorValue := messengerPrograms(repository, packageTargets[0])

	requireRefusalNamesBothRevisions(t, errorValue, revisionThisTreeBuilds(t, repository),
		relayRevisionBeforeTheDeclaredMimePatch, "tools/prepare-buzz-relay --target linux-arm64")
}

func TestALinuxPackageCarriesARelayBuiltAtTheTreesRevision(t *testing.T) {
	repository := linuxMessengerRepository(t)
	recordMessengerRevision(t, repository, blueclaw.BuzzRelayArtifactPath, revisionThisTreeBuilds(t, repository))

	packaged, errorValue := messengerPrograms(repository, packageTargets[0])

	if errorValue != nil {
		t.Fatalf("a relay built at this tree's revision was refused: %v", errorValue)
	}
	if len(packaged) != len(messengerProgramNames) {
		t.Errorf("the package carries %d messenger programs and there are %d", len(packaged), len(messengerProgramNames))
	}
}

func TestALinuxPackageRefusesARelayThatRecordsNoRevision(t *testing.T) {
	repository := linuxMessengerRepository(t)

	_, errorValue := messengerPrograms(repository, packageTargets[0])

	if errorValue == nil || !strings.Contains(errorValue.Error(), "record no revision") {
		t.Fatalf("a relay with no REVISION was packaged: %v", errorValue)
	}
}

func TestAKegRefusesARelayBuiltBeforeTheTreesPatches(t *testing.T) {
	repository := macMessengerRepository(t)
	recordMessengerRevision(t, repository, brewMessengerArtifactPath, relayRevisionBeforeTheDeclaredMimePatch)

	errorValue := copyBrewMessengerPrograms(repository, t.TempDir(), io.Discard)

	requireRefusalNamesBothRevisions(t, errorValue, revisionThisTreeBuilds(t, repository),
		relayRevisionBeforeTheDeclaredMimePatch, "tools/prepare-buzz-relay --target darwin-arm64")
}

func TestAKegCarriesARelayBuiltAtTheTreesRevision(t *testing.T) {
	repository := macMessengerRepository(t)
	recordMessengerRevision(t, repository, brewMessengerArtifactPath, revisionThisTreeBuilds(t, repository))
	libraryPath := t.TempDir()

	if errorValue := copyBrewMessengerPrograms(repository, libraryPath, io.Discard); errorValue != nil {
		t.Fatalf("a relay built at this tree's revision was refused: %v", errorValue)
	}
	for _, name := range messengerProgramNames {
		if _, errorValue := os.Stat(filepath.Join(libraryPath, name)); errorValue != nil {
			t.Errorf("the keg does not carry %s: %v", name, errorValue)
		}
	}
}
