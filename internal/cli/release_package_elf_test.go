package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestTheMessengerDirectoriesAreTheOnesThePrepareScriptFills(t *testing.T) {
	script, readError := os.ReadFile("../../tools/prepare-buzz-relay")
	if readError != nil {
		t.Skip("the prepare script is not in this tree")
	}
	for _, architecture := range []string{"arm64", "amd64"} {
		directory := blueclaw.BuzzRelayArtifactPathFor(architecture)
		if !strings.Contains(string(script), "$repository_root/"+directory+"\"") {
			t.Errorf("tools/prepare-buzz-relay does not build %s into %s", architecture, directory)
		}
	}
}

func TestGlibcVersionsOrderNumerically(t *testing.T) {
	older, _ := parseGlibcVersion("GLIBC_2.9")
	newer, _ := parseGlibcVersion("GLIBC_2.35")
	if compareVersions(older, newer) >= 0 {
		t.Fatal("GLIBC_2.9 sorted after GLIBC_2.35")
	}
	if _, isGlibc := parseGlibcVersion("GLIBC_PRIVATE"); isGlibc {
		t.Fatal("GLIBC_PRIVATE was read as a version")
	}
}

func compileLinuxProgram(t *testing.T, architecture string) string {
	t.Helper()
	directory := t.TempDir()
	source := filepath.Join(directory, "main.go")
	if writeError := os.WriteFile(source, []byte("package main\nfunc main() {}\n"), 0o644); writeError != nil {
		t.Fatal(writeError)
	}
	built := filepath.Join(directory, "program")
	command := exec.Command("go", "build", "-o", built, source)
	command.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+architecture, "CGO_ENABLED=0", "GO111MODULE=off")
	if output, buildError := command.CombinedOutput(); buildError != nil {
		t.Fatalf("compile a linux/%s program: %s", architecture, output)
	}
	return built
}

func TestAnotherArchitecturesMessengerIsRefused(t *testing.T) {
	amd64Program := compileLinuxProgram(t, "amd64")
	errorValue := requireMessengerBinary(amd64Program, "buzz-relay", packageTargets[0])
	if errorValue == nil || !strings.Contains(errorValue.Error(), "--target linux-arm64") {
		t.Fatalf("an amd64 binary was accepted for the arm64 package: %v", errorValue)
	}
	if errorValue := requireMessengerBinary(amd64Program, "buzz-relay", packageTargets[1]); errorValue != nil {
		t.Fatalf("an amd64 binary was refused for the amd64 package: %v", errorValue)
	}
}

func TestAStaticProgramFitsItsOwnArchitectureOnly(t *testing.T) {
	amd64Program := compileLinuxProgram(t, "amd64")
	if errorValue := requireELFFits(amd64Program, packageTargets[1]); errorValue != nil {
		t.Fatalf("a static amd64 program does not fit the amd64 package: %v", errorValue)
	}
	if errorValue := requireELFFits(amd64Program, packageTargets[0]); errorValue == nil {
		t.Fatal("a static amd64 program fit the arm64 package")
	}
}
