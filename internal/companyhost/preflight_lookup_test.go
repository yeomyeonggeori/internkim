package companyhost

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type aMachineThatSearchesPath struct {
	recordedMachine
}

func (aMachineThatSearchesPath) CarriesProgram(programName string) error {
	_, errorValue := exec.LookPath(programName)
	return errorValue
}

func (aMachineThatSearchesPath) CarriesFile(path string) error {
	_, errorValue := os.Stat(path)
	return errorValue
}

func placeStubPrograms(t *testing.T, directory string, names []string) {
	t.Helper()
	if errorValue := os.MkdirAll(directory, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, name := range names {
		if errorValue := os.WriteFile(filepath.Join(directory, name), []byte("#!/bin/sh\n"), 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func whatTheOperatingSystemProvides() []string {
	programs := []string{}
	for _, dependency := range blueclaw.HostDependencies() {
		if dependency.ArrivesAsPayload || dependency.OnlyTheImageEntrypointRuns() {
			continue
		}
		programs = append(programs, dependency.ProgramsTheHostRuns...)
		programs = append(programs, dependency.OneOfThesePrograms...)
	}
	return programs
}

func TestAKegFindsThePackagesProgramsWhereTheLayoutPutsThemAndNotOnPath(t *testing.T) {
	prefix := t.TempDir()
	platform := macPlatform{homebrewPrefix: prefix}
	layout := platform.Layout()
	placeStubPrograms(t, layout.BinaryRoot, append(blueclaw.HostProgramsThePackageShips(), blueclaw.HostProgramsThatArriveAsPayload()...))
	placeStubPrograms(t, filepath.Join(prefix, "bin"), append(whatTheOperatingSystemProvides(), blueclaw.CompanyPackageName))
	t.Setenv("PATH", filepath.Join(prefix, "bin"))

	if errorValue := requireWhatTheCompanyHostRuns(platform, &aMachineThatSearchesPath{}); errorValue != nil {
		t.Fatalf("a keg holding every program in %s was refused, because the preflight searched PATH for them:\n%v", layout.BinaryRoot, errorValue)
	}
}

func TestAProgramThePackageShipsIsMissingWhenTheLayoutDoesNotHoldIt(t *testing.T) {
	prefix := t.TempDir()
	platform := macPlatform{homebrewPrefix: prefix}
	layout := platform.Layout()
	placeStubPrograms(t, filepath.Join(prefix, "bin"), append(whatTheOperatingSystemProvides(), blueclaw.HostProgramsThePackageShips()...))
	placeStubPrograms(t, filepath.Join(prefix, "bin"), blueclaw.HostProgramsThatArriveAsPayload())
	t.Setenv("PATH", filepath.Join(prefix, "bin"))

	errorValue := requireWhatTheCompanyHostRuns(platform, &aMachineThatSearchesPath{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), blueclaw.CapabilitydName) {
		t.Fatalf("programs on PATH stood in for the ones %s should hold: %v", layout.BinaryRoot, errorValue)
	}
}
