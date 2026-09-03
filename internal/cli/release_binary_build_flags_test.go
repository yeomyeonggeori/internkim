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
