package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const exampleManifest = `+internkim
# the archive key signs what every company host installs as root
@release
INTERNKIM_APT_SIGNING_KEY
@cli,scripts
INTERNKIM_REGISTER_SECRET
INTERNKIM_DOMAIN
INTERNKIM_BOARD_PORT=8080
@scripts
CLOUDFLARE_API_TOKEN
`

func TestManifestNamesAreTheBareKeysOfTheProfilesThatListTheName(t *testing.T) {
	names := vaultManifestNames(exampleManifest, "cli")
	wanted := []string{"INTERNKIM_REGISTER_SECRET", "INTERNKIM_DOMAIN"}
	if !slices.Equal(names, wanted) {
		t.Fatalf("the cli profile read %v, wanted %v", names, wanted)
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
	if names := vaultManifestNames(exampleManifest, "cli"); slices.Contains(names, "INTERNKIM_BOARD_PORT") {
		t.Fatalf("a manifest value was counted as a vault secret: %v", names)
	}
}

func TestTheGapIsTheProfilesOwnMissingLine(t *testing.T) {
	doctorOutput := "missing @release: INTERNKIM_APT_SIGNING_KEY\nmissing @cli: INTERNKIM_REGISTER_SECRET, INTERNKIM_DOMAIN\n"
	missing := vaultMissingNames(doctorOutput, "cli")
	wanted := []string{"INTERNKIM_REGISTER_SECRET", "INTERNKIM_DOMAIN"}
	if !slices.Equal(missing, wanted) {
		t.Fatalf("the cli gap read %v, wanted %v", missing, wanted)
	}
}

func TestAProfileWithNoGapReportsNothing(t *testing.T) {
	if missing := vaultMissingNames("missing @release: INTERNKIM_APT_SIGNING_KEY\n", "cli"); len(missing) != 0 {
		t.Fatalf("a profile with nothing missing reported %v", missing)
	}
	if missing := vaultMissingNames("", "cli"); len(missing) != 0 {
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

	if _, shouldRun := plannedVaultRun(); shouldRun {
		t.Fatal("a directory that is not a checkout planned a run through the vault")
	}
}

// A checkout whose manifest opens no cli profile is every commit before the
// values moved: the CLI runs exactly as it did, and nothing is asked of the
// vault.
func TestAManifestWithoutTheProfileIsNotConsulted(t *testing.T) {
	repositoryRootPath := t.TempDir()
	writeFile(t, filepath.Join(repositoryRootPath, "go.mod"), "module example.test\n")
	writeFile(t, filepath.Join(repositoryRootPath, vaultManifestName), "+internkim\n@release\nINTERNKIM_APT_SIGNING_KEY\n")
	t.Chdir(repositoryRootPath)
	t.Setenv("INTERNKIM_ENVIRONMENT_FROM_VAULT", "")

	if _, shouldRun := plannedVaultRun(); shouldRun {
		t.Fatal("a manifest with no cli profile planned a run through the vault")
	}
}

// The re-executed run must not plan another one, whatever the manifest says.
func TestTheReExecutedRunDoesNotRunThroughTheVaultAgain(t *testing.T) {
	repositoryRootPath := t.TempDir()
	writeFile(t, filepath.Join(repositoryRootPath, "go.mod"), "module example.test\n")
	writeFile(t, filepath.Join(repositoryRootPath, vaultManifestName), exampleManifest)
	t.Chdir(repositoryRootPath)
	t.Setenv("INTERNKIM_ENVIRONMENT_FROM_VAULT", "1")

	if _, shouldRun := plannedVaultRun(); shouldRun {
		t.Fatal("the run that already has the vault environment planned another one")
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if errorValue := os.WriteFile(path, []byte(contents), 0o600); errorValue != nil {
		t.Fatalf("write %s: %v", path, errorValue)
	}
}
