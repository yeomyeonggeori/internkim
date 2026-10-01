package cli

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestTheFormulaFetchesTheBottleAndTheTarballFromItsOwnRelease(t *testing.T) {
	formula, errorValue := homebrewReleaseFormula("2026.10.01.090507", "source-sum", homebrewBottle("arm64_ventura", "bottle-sum"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	download := "https://github.com/yeomyeonggeori/internkim/releases/download/v2026.10.01.090507"
	for _, line := range []string{
		`  url "` + download + `/internkim-macos-arm64.tar.gz"`,
		`    root_url "` + download + `"`,
		`    sha256 cellar: :any_skip_relocation, arm64_ventura: "bottle-sum"`,
	} {
		if !strings.Contains(formula, line+"\n") {
			t.Errorf("the formula does not carry %s:\n%s", line, formula)
		}
	}
	if strings.Contains(formula, "updates.") {
		t.Errorf("the formula still names the release registry:\n%s", formula)
	}
	if name := blueclaw.HomebrewBottleFileName("2026.10.01.090507", "arm64_ventura"); name != "internkim-2026.10.01.090507.arm64_ventura.bottle.tar.gz" {
		t.Errorf("Homebrew asks root_url for internkim-<version>.<tag>.bottle.tar.gz, and the release names it %s", name)
	}
}

// writeMachOForTest writes the smallest thin arm64 Mach-O debug/macho reads: a
// header and one LC_BUILD_VERSION naming macOS at the given major version.
func writeMachOForTest(t *testing.T, path string, macOSMajor uint32) {
	t.Helper()
	words := []uint32{
		0xfeedfacf, 0x0100000c, 0, 2, 1, 24, 0, 0,
		loadCommandBuildVersion, 24, machOPlatformMacOS, macOSMajor << 16, macOSMajor << 16, 0,
	}
	document := make([]byte, 4*len(words))
	for index, word := range words {
		binary.LittleEndian.PutUint32(document[4*index:], word)
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, document, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestTheBottleIsNamedForTheOldestMacOSEveryProgramInTheKegRunsOn(t *testing.T) {
	keg := t.TempDir()
	writeMachOForTest(t, filepath.Join(keg, "bin", "internkim"), 12)
	writeMachOForTest(t, filepath.Join(keg, "libexec", "bun"), 13)
	writeMachOForTest(t, filepath.Join(keg, "libexec", "buzz-relay"), 11)
	writeFileForTest(t, filepath.Join(keg, "libexec", "render-company-runtime"), "#!/bin/sh\n")
	writeFileForTest(t, filepath.Join(keg, "libexec", "skills", "SKILL.md"), "# a skill\n")

	minimum, errorValue := kegMinimumMacOSVersion(keg)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if minimum != 13 {
		t.Errorf("the keg needs macOS 13 for bun and was read as %d", minimum)
	}
}

func TestAKegWithNoMacOSProgramHasNoBottleTag(t *testing.T) {
	keg := t.TempDir()
	writeFileForTest(t, filepath.Join(keg, "libexec", "render-company-runtime"), "#!/bin/sh\n")
	if _, errorValue := kegMinimumMacOSVersion(keg); errorValue == nil {
		t.Fatal("a keg with no Mach-O program was given a minimum macOS")
	}
}
