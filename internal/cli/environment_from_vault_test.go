package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const exampleManifest = `+internkim
# the archive key signs what every company host installs as root
@production,scripts
INTERNKIM_APT_SIGNING_KEY
INTERNKIM_REGISTER_SECRET
INTERNKIM_DOMAIN
INTERNKIM_BOARD_PORT=8080
@scripts
CLOUDFLARE_API_TOKEN
`

// Nothing in this repository writes the profile's name but the manifest, so
// what the manifest declares first is what every `monkeys` call runs under.
func TestTheProfileIsTheFirstOneTheManifestDeclares(t *testing.T) {
	if profile := vaultManifestProfile(exampleManifest); profile != "production" {
		t.Fatalf("the manifest named %q, wanted the first profile it declares", profile)
	}
}

func TestAManifestThatDeclaresNoProfileNamesNone(t *testing.T) {
	if profile := vaultManifestProfile("+internkim\n"); profile != "" {
		t.Fatalf("a manifest with no profile named %q", profile)
	}
}

func TestManifestNamesAreTheBareKeysOfTheProfilesThatListTheName(t *testing.T) {
	names := vaultManifestNames(exampleManifest, "production")
	wanted := []string{"INTERNKIM_APT_SIGNING_KEY", "INTERNKIM_REGISTER_SECRET", "INTERNKIM_DOMAIN"}
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
	doctorOutput := "missing @scripts: CLOUDFLARE_API_TOKEN\nmissing @production: INTERNKIM_REGISTER_SECRET, INTERNKIM_DOMAIN\n"
	missing := vaultMissingNames(doctorOutput, "production")
	wanted := []string{"INTERNKIM_REGISTER_SECRET", "INTERNKIM_DOMAIN"}
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

	if _, shouldRun := plannedVaultRun(); shouldRun {
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

	if _, shouldRun := plannedVaultRun(); shouldRun {
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
