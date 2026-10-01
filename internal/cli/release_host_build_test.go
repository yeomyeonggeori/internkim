package cli

import (
	"strings"
	"testing"
)

const releasedHead = "f5ef4632f41f4c02cca29262bea5399aaae3f04a"

func TestACLIBuiltFromTheTreesCleanHeadMayRelease(t *testing.T) {
	if reason := staleBuildReason(cliBuild{Revision: releasedHead}, releasedHead); reason != "" {
		t.Fatalf("a CLI built from HEAD was refused: %s", reason)
	}
}

func TestACLIBuiltFromAnotherCommitIsRefused(t *testing.T) {
	reason := staleBuildReason(cliBuild{Revision: "3d83c0bde492521dcc02fcb4379fd0aff6140d9c"}, releasedHead)
	if !strings.Contains(reason, "3d83c0b") || !strings.Contains(reason, "make build") {
		t.Fatalf("a CLI from another commit must be refused by name with the remedy, got %q", reason)
	}
}

func TestACLIBuiltFromADirtyTreeOrWithoutACommitIsRefused(t *testing.T) {
	for _, build := range []cliBuild{{Revision: releasedHead, IsModified: true}, {}} {
		if staleBuildReason(build, releasedHead) == "" {
			t.Errorf("%+v was allowed to release", build)
		}
	}
}
