package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const exampleManifest = `+internkim
@test
OPENROUTER_API_KEY
# the archive key signs what every company host installs as root
@production,scripts
INTERNKIM_APT_SIGNING_KEY
INTERNKIM_REGISTER_SECRET
CLOUDFLARE_DOMAIN
INTERNKIM_BOARD_PORT=8080
@scripts
CLOUDFLARE_API_TOKEN
`

func TestWithNoProfileArgumentTheFirstDeclaredProfileIsChosen(t *testing.T) {
	profile, errorValue := chosenVaultProfile(exampleManifest, "")
	if errorValue != nil || profile != "test" {
		t.Fatalf("chose %q (%v), wanted the first profile the manifest declares", profile, errorValue)
	}
}

func TestAProfileArgumentChoosesThatProfile(t *testing.T) {
	profile, errorValue := chosenVaultProfile(exampleManifest, "production")
	if errorValue != nil || profile != "production" {
		t.Fatalf("chose %q (%v), wanted production", profile, errorValue)
	}
}

func TestAProfileTheManifestDoesNotDeclareIsRefused(t *testing.T) {
	if _, errorValue := chosenVaultProfile(exampleManifest, "prodution"); errorValue == nil {
		t.Fatal("a misspelled profile was accepted and would have run with no vault values")
	}
}

func TestAManifestThatDeclaresNoProfileChoosesNone(t *testing.T) {
	profile, errorValue := chosenVaultProfile("+internkim\n", "")
	if errorValue != nil || profile != "" {
		t.Fatalf("a manifest with no profile chose %q (%v)", profile, errorValue)
	}
}

func TestTheDeclaredProfilesAreListedOnceInOrder(t *testing.T) {
	wanted := []string{"test", "production", "scripts"}
	if declared := vaultManifestProfiles(exampleManifest); !slices.Equal(declared, wanted) {
		t.Fatalf("declared %v, wanted %v", declared, wanted)
	}
}

func TestALeadingProfileArgumentIsTakenOffTheCommand(t *testing.T) {
	profile, rest := splitVaultProfileArgument([]string{"@production", "deploy", "--components", "web"})
	if profile != "production" || !slices.Equal(rest, []string{"deploy", "--components", "web"}) {
		t.Fatalf("split into %q and %v", profile, rest)
	}
	profile, rest = splitVaultProfileArgument([]string{"deploy", "@production"})
	if profile != "" || !slices.Equal(rest, []string{"deploy", "@production"}) {
		t.Fatalf("a profile after the command was taken as the vault profile: %q, %v", profile, rest)
	}
}

func TestManifestNamesAreTheBareKeysOfTheProfilesThatListTheName(t *testing.T) {
	names := vaultManifestNames(exampleManifest, "production")
	wanted := []string{"INTERNKIM_APT_SIGNING_KEY", "INTERNKIM_REGISTER_SECRET", "CLOUDFLARE_DOMAIN"}
	if !slices.Equal(names, wanted) {
		t.Fatalf("the production profile read %v, wanted %v", names, wanted)
	}
}

func TestAProfileTheManifestDoesNotOpenHasNoNames(t *testing.T) {
	if names := vaultManifestNames(exampleManifest, "companion"); len(names) != 0 {
		t.Fatalf("a profile nothing declares read %v", names)
	}
}

// A `KEY=value` line is a value that is not secret. `monkeys run` reads it
// from the manifest itself, so asking the vault for it would always come back
// empty.
func TestAValueInTheManifestIsNotAskedOfTheVault(t *testing.T) {
	if names := vaultManifestNames(exampleManifest, "production"); slices.Contains(names, "INTERNKIM_BOARD_PORT") {
		t.Fatalf("a manifest value was counted as a vault secret: %v", names)
	}
}

func TestTheGapIsTheProfilesOwnMissingLine(t *testing.T) {
	doctorOutput := "missing @scripts: CLOUDFLARE_API_TOKEN\nmissing @production: INTERNKIM_REGISTER_SECRET, CLOUDFLARE_DOMAIN\n"
	missing := vaultMissingNames(doctorOutput, "production")
	wanted := []string{"INTERNKIM_REGISTER_SECRET", "CLOUDFLARE_DOMAIN"}
	if !slices.Equal(missing, wanted) {
		t.Fatalf("the production gap read %v, wanted %v", missing, wanted)
	}
}

func TestAProfileWithNoGapReportsNothing(t *testing.T) {
	if missing := vaultMissingNames("missing @scripts: CLOUDFLARE_API_TOKEN\n", "production"); len(missing) != 0 {
		t.Fatalf("a profile with nothing missing reported %v", missing)
	}
	if missing := vaultMissingNames("", "production"); len(missing) != 0 {
		t.Fatalf("an empty doctor report read %v", missing)
	}
}

// The marker is read as a literal so `tools/verify-environment-declarations`
// can see the read, and set from the constant when the CLI re-executes itself.
// The two have to stay the same name.
func TestTheMarkerIsTheNameTheReExecutedRunLooksFor(t *testing.T) {
	if vaultInjectedMarker != "INTERNKIM_ENVIRONMENT_FROM_VAULT" {
		t.Fatalf("the marker is %q, which is not the name the guard reads", vaultInjectedMarker)
	}
}

// Outside a checkout there is no manifest to read, which is how the CLI an
// installed package ships behaves: it takes the environment it was given.
func TestOutsideARepositoryTheVaultIsNotConsulted(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("INTERNKIM_ENVIRONMENT_FROM_VAULT", "")

	if _, shouldRun, _ := plannedVaultRun(""); shouldRun {
		t.Fatal("a directory that is not a checkout planned a run through the vault")
	}
}

// A checkout whose manifest declares no profile is every commit before the
// values moved: the CLI runs exactly as it did, and nothing is asked of the
// vault.
func TestAManifestWithoutAProfileIsNotConsulted(t *testing.T) {
	repositoryRootPath := t.TempDir()
	writeFile(t, filepath.Join(repositoryRootPath, "go.mod"), "module example.test\n")
	writeFile(t, filepath.Join(repositoryRootPath, vaultManifestName), "+internkim\n")
	t.Chdir(repositoryRootPath)
	t.Setenv("INTERNKIM_ENVIRONMENT_FROM_VAULT", "")

	if _, shouldRun, _ := plannedVaultRun(""); shouldRun {
		t.Fatal("a manifest that declares no profile planned a run through the vault")
	}
}

// The re-executed run must not plan another one, whatever the manifest says.
func TestTheReExecutedRunDoesNotRunThroughTheVaultAgain(t *testing.T) {
	repositoryRootPath := t.TempDir()
	writeFile(t, filepath.Join(repositoryRootPath, "go.mod"), "module example.test\n")
	writeFile(t, filepath.Join(repositoryRootPath, vaultManifestName), exampleManifest)
	t.Chdir(repositoryRootPath)
	t.Setenv("INTERNKIM_ENVIRONMENT_FROM_VAULT", "1")

	if _, shouldRun, _ := plannedVaultRun(""); shouldRun {
		t.Fatal("the run that already has the vault environment planned another one")
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if errorValue := os.WriteFile(path, []byte(contents), 0o600); errorValue != nil {
		t.Fatalf("write %s: %v", path, errorValue)
	}
}

func TestAProfileTheVaultCannotFillStopsTheCommand(t *testing.T) {
	repositoryRootPath := checkoutWithAFakeVault(t, "echo 'missing @test: FIRST_KEY, SECOND_KEY'")

	_, shouldRun, errorValue := plannedVaultRun("")

	if shouldRun || errorValue == nil {
		t.Fatalf("a vault missing two declared keys let the command run in %s", repositoryRootPath)
	}
	if !strings.Contains(errorValue.Error(), "FIRST_KEY, SECOND_KEY") {
		t.Fatalf("the refusal does not name what is missing: %v", errorValue)
	}
}

func TestAVaultThatCannotSayWhatItLacksStopsTheCommand(t *testing.T) {
	checkoutWithAFakeVault(t, "exit 3")

	if _, shouldRun, errorValue := plannedVaultRun(""); shouldRun || errorValue == nil {
		t.Fatal("a doctor that failed was read as a vault with nothing missing")
	}
}

func TestADeclaredProfileWithoutMonkeysStopsTheCommand(t *testing.T) {
	repositoryRootPath := t.TempDir()
	writeFile(t, filepath.Join(repositoryRootPath, "go.mod"), "module example.test\n")
	writeFile(t, filepath.Join(repositoryRootPath, vaultManifestName), exampleManifest)
	t.Chdir(repositoryRootPath)
	t.Setenv("INTERNKIM_ENVIRONMENT_FROM_VAULT", "")
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	if _, shouldRun, errorValue := plannedVaultRun(""); shouldRun || errorValue == nil {
		t.Fatal("a manifest that declares secrets ran without monkeys to supply them")
	}
}

func TestAFullVaultIsRunThrough(t *testing.T) {
	checkoutWithAFakeVault(t, "true")

	plan, shouldRun, errorValue := plannedVaultRun("")

	if errorValue != nil || !shouldRun || plan.profile != "test" {
		t.Fatalf("a vault with nothing missing was not run through: %+v %v %v", plan, shouldRun, errorValue)
	}
}

func checkoutWithAFakeVault(t *testing.T, doctorBody string) string {
	t.Helper()
	repositoryRootPath := t.TempDir()
	writeFile(t, filepath.Join(repositoryRootPath, "go.mod"), "module example.test\n")
	writeFile(t, filepath.Join(repositoryRootPath, vaultManifestName), exampleManifest)
	binaryDirectory := t.TempDir()
	monkeysPath := filepath.Join(binaryDirectory, "monkeys")
	if errorValue := os.WriteFile(monkeysPath, []byte("#!/bin/sh\n"+doctorBody+"\n"), 0o755); errorValue != nil {
		t.Fatalf("write the fake monkeys: %v", errorValue)
	}
	t.Chdir(repositoryRootPath)
	t.Setenv("INTERNKIM_ENVIRONMENT_FROM_VAULT", "")
	t.Setenv("PATH", binaryDirectory)
	return repositoryRootPath
}
