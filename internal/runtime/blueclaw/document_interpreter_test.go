package blueclaw_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestTheHostImageBuildsTheConversionEnvironmentTheWayThePackageDoes(t *testing.T) {
	document, readError := os.ReadFile(filepath.Join(repositoryRootFromHere, "host", "Dockerfile"))
	if readError != nil {
		t.Fatal(readError)
	}
	dockerfile := strings.Join(logicalLinesOf(string(document)), "\n")
	layout := blueclaw.DebianCompanyHostLayout()
	for _, command := range layout.DocumentEnvironmentCommands()[:2] {
		inTheImage := strings.ReplaceAll(strings.Join(command.Arguments, " "), layout.BinaryPath(blueclaw.PackageResolverName), blueclaw.PackageResolverName)
		if !strings.Contains(dockerfile, inTheImage) {
			t.Errorf("host/Dockerfile does not run %q, which is how the package builds the conversion environment", inTheImage)
		}
	}
}

func TestEveryRequirementTheConversionNamesIsLocked(t *testing.T) {
	directory := filepath.Join(repositoryRootFromHere, "assets", "document-conversion")
	named, readError := os.ReadFile(filepath.Join(directory, "requirements.in"))
	if readError != nil {
		t.Fatal(readError)
	}
	locked, readError := os.ReadFile(filepath.Join(directory, "requirements.txt"))
	if readError != nil {
		t.Fatal(readError)
	}
	for _, name := range strings.Fields(string(named)) {
		if !strings.Contains(string(locked), "\n"+name+"==") {
			t.Errorf("requirements.in names %s and requirements.txt pins no version of it; regenerate the lock with the command its header records", name)
		}
	}
	for _, line := range strings.Split(string(locked), "\n") {
		if strings.Contains(line, "==") && !strings.HasPrefix(line, " ") && !strings.HasSuffix(line, "\\") {
			t.Errorf("%q carries no hash, and the install step syncs with --require-hashes", line)
		}
	}
}

func TestTheFormulasPostInstallBuildsTheConversionEnvironmentThroughTheOptLink(t *testing.T) {
	formula, errorValue := blueclaw.HomebrewFormula(blueclaw.HomebrewFormulaRequest{Version: "1.2.3", SourceSHA256: "checksum", SourceTarballURL: "https://example.com/internkim.tar.gz"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	postInstall := formula[strings.Index(formula, "def post_install"):strings.Index(formula, "def caveats")]
	layout := blueclaw.MacCompanyHostLayout("#{HOMEBREW_PREFIX}")
	for _, command := range layout.DocumentEnvironmentCommands() {
		quoted := `"` + strings.Join(command.Arguments, `", "`) + `"`
		if !strings.Contains(postInstall, "system "+quoted) {
			t.Fatalf("post_install does not %s:\n%s", command.Purpose, postInstall)
		}
	}
	if strings.Contains(postInstall, "return if") {
		t.Fatalf("post_install skips itself, so `brew postinstall` cannot repair an environment:\n%s", postInstall)
	}
}
