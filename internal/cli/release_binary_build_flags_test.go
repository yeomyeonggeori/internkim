package cli

import (
	"strings"
	"testing"
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

func TestAdmindIsBuiltWithTheCentralPlaneTheVaultNames(t *testing.T) {
	t.Setenv("SUPABASE_URL", "https://plane.example.test")
	t.Setenv("SUPABASE_PUBLISHABLE_KEY", "publishable-for-the-test")

	stamped, errorValue := admindStampFlags("build", "revision")
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

func TestAdmindIsNotBuiltWhenTheVaultNamesNoCentralPlane(t *testing.T) {
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_PUBLISHABLE_KEY", "")

	_, errorValue := admindStampFlags("build", "revision")
	if errorValue == nil {
		t.Fatal("admind was built with no central plane to default to")
	}
	for _, wanted := range []string{"SUPABASE_URL", "SUPABASE_PUBLISHABLE_KEY", "monkeys run @production"} {
		if !strings.Contains(errorValue.Error(), wanted) {
			t.Fatalf("the refusal %q does not name %s", errorValue, wanted)
		}
	}
}
