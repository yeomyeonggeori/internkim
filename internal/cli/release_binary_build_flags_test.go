package cli

import (
	"strings"
	"testing"
)

func TestReleaseBinaryBuildFlagsCarryTheCheckedOutRevision(t *testing.T) {
	flags := releaseBinaryBuildFlags("../..")
	if len(flags) != 2 || flags[0] != "-ldflags" {
		t.Fatalf("expected a single -ldflags argument, got %v", flags)
	}
	if strings.Contains(flags[1], "GitRevision=unknown") || !strings.Contains(flags[1], "internal/admind.GitRevision=") {
		t.Fatalf("expected the release binary to carry the checked-out revision, got %q", flags[1])
	}
}

func TestAdmindIsBuiltWithTheCentralPlaneTheVaultNames(t *testing.T) {
	t.Setenv("SUPABASE_URL", "https://plane.example.test")
	t.Setenv("SUPABASE_PUBLISHABLE_KEY", "publishable-for-the-test")

	stamped := admindStampFlags("build", "revision")
	for _, required := range []string{
		"internal/centralplane.DefaultProjectURL=https://plane.example.test",
		"internal/centralplane.DefaultPublishableKey=publishable-for-the-test",
	} {
		if !strings.Contains(stamped, required) {
			t.Fatalf("admind built with %q does not carry %s", stamped, required)
		}
	}
}

func TestAdmindIsBuiltWithoutACentralPlaneWhenTheVaultNamesNone(t *testing.T) {
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_PUBLISHABLE_KEY", "")

	if stamped := admindStampFlags("build", "revision"); strings.Contains(stamped, "centralplane") {
		t.Fatalf("admind was stamped with a central plane nothing named: %q", stamped)
	}
}
