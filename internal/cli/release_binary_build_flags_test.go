package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestReleaseBinaryBuildFlagsCarryTheCheckedOutRevision(t *testing.T) {
	t.Setenv("SUPABASE_URL", "https://plane.example.test")
	t.Setenv("SUPABASE_PUBLISHABLE_KEY", "publishable-for-the-test")
	flags, errorValue := releaseBinaryBuildFlags("../..")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(flags) != 2 || flags[0] != "-ldflags" {
		t.Fatalf("expected a single -ldflags argument, got %v", flags)
	}
	if strings.Contains(flags[1], "GitRevision=unknown") || !strings.Contains(flags[1], "internal/admind.GitRevision=") {
		t.Fatalf("expected the release binary to carry the checked-out revision, got %q", flags[1])
	}
}

func TestTheDevicesAdmindIsBuiltWithTheCentralPlaneTheVaultNames(t *testing.T) {
	t.Setenv("SUPABASE_URL", "https://plane.example.test")
	t.Setenv("SUPABASE_PUBLISHABLE_KEY", "publishable-for-the-test")

	stamped, errorValue := deviceAdmindStampFlags("build", "revision")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, required := range []string{
		"internal/centralplane.DefaultProjectURL=https://plane.example.test",
		"internal/centralplane.DefaultPublishableKey=publishable-for-the-test",
	} {
		if !strings.Contains(stamped, required) {
			t.Fatalf("admind built with %q does not carry %s", stamped, required)
		}
	}
}

func TestTheDevicesAdmindIsNotBuiltWhenTheVaultNamesNoCentralPlane(t *testing.T) {
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_PUBLISHABLE_KEY", "")

	_, errorValue := deviceAdmindStampFlags("build", "revision")
	if errorValue == nil {
		t.Fatal("the device's admind was built with no central plane to default to")
	}
	for _, wanted := range []string{"SUPABASE_URL", "SUPABASE_PUBLISHABLE_KEY", "monkeys run @production"} {
		if !strings.Contains(errorValue.Error(), wanted) {
			t.Fatalf("the refusal %q does not name %s", errorValue, wanted)
		}
	}
}

func TestAHostPackagesAdmindBuildsFromACloneWithNoVault(t *testing.T) {
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_PUBLISHABLE_KEY", "")
	if stamped := admindStampFlags("build", "revision"); strings.Contains(stamped, "centralplane") {
		t.Fatalf("a host package's admind carries a central plane: %s", stamped)
	}

	repositoryRootPath, errorValue := filepath.Abs("../..")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	admind := packagedGoProgram{Name: blueclaw.AdmindName, Package: "./cmd/" + blueclaw.AdmindName}
	outputPath := filepath.Join(t.TempDir(), blueclaw.AdmindName)
	if errorValue := crossCompilePackagedProgram(repositoryRootPath, admind, packageTargets[0], "1.2.3", outputPath); errorValue != nil {
		t.Fatalf("release packages could not build admind with no SUPABASE_URL or SUPABASE_PUBLISHABLE_KEY: %v", errorValue)
	}
}
