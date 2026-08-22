package cli

import (
	"slices"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/releaseset"
)

func carriedComponents(t *testing.T, rebuilt map[string]releaseset.Component, published map[string]releaseset.Component, revisionOf func(string) string) (map[string]releaseset.Component, error) {
	t.Helper()
	return rebuilt, carryComponentsForward(rebuilt, published, revisionOf)
}

func TestAComponentThatStillMatchesTheTreeIsCarriedForward(t *testing.T) {
	rebuilt := map[string]releaseset.Component{"relay": {Name: "relay", Revision: "tree-1"}}
	published := map[string]releaseset.Component{
		"relay": {Name: "relay", Revision: "old"},
		"chatd": {Name: "chatd", Revision: "blueclaw-1"},
	}

	carried, errorValue := carriedComponents(t, rebuilt, published, func(name string) string {
		return map[string]string{"relay": "tree-1", "chatd": "blueclaw-1"}[name]
	})
	if errorValue != nil {
		t.Fatalf("nothing had moved, yet: %v", errorValue)
	}
	if carried["chatd"].Revision != "blueclaw-1" {
		t.Fatalf("the release must still name chatd, got %+v", carried)
	}
}

func TestAComponentTheTreeHasMovedPastStopsTheRelease(t *testing.T) {
	rebuilt := map[string]releaseset.Component{"relay": {Name: "relay", Revision: "tree-2"}}
	published := map[string]releaseset.Component{
		"relay": {Name: "relay", Revision: "tree-1"},
		"chatd": {Name: "chatd", Revision: "blueclaw-1"},
	}

	_, errorValue := carriedComponents(t, rebuilt, published, func(name string) string {
		return map[string]string{"relay": "tree-2", "chatd": "blueclaw-2"}[name]
	})
	if errorValue == nil {
		t.Fatal("a relay shipped beside a chatd built from an older blueclaw was accepted")
	}
	if !strings.Contains(errorValue.Error(), "chatd") {
		t.Fatalf("the refusal must name the component to rebuild: %v", errorValue)
	}
}

func TestTheFirstReleaseOfAComponentIsNotMissed(t *testing.T) {
	rebuilt := map[string]releaseset.Component{"relay": {Name: "relay", Revision: "tree-1"}}
	published := map[string]releaseset.Component{"relay": {Name: "relay", Revision: "tree-1"}}

	carried, errorValue := carriedComponents(t, rebuilt, published, func(string) string { return "tree-1" })
	if errorValue != nil {
		t.Fatalf("a component the device has never had must not stop a release: %v", errorValue)
	}
	if _, named := carried["chatd"]; named {
		t.Fatal("a component that was never published cannot be carried forward")
	}
}

func TestAComponentThisTreeCannotBuildIsCarriedRatherThanBlocking(t *testing.T) {
	rebuilt := map[string]releaseset.Component{"chatd": {Name: "chatd", Revision: "blueclaw-2"}}
	published := map[string]releaseset.Component{
		"chatd":           {Name: "chatd", Revision: "blueclaw-1"},
		"blueclawPayload": {Name: "blueclawPayload", Revision: "payload-7"},
	}

	// The payload artifact is not in this tree, so it has no revision to offer.
	carried, errorValue := carriedComponents(t, rebuilt, published, func(name string) string {
		return map[string]string{"chatd": "blueclaw-2", "blueclawPayload": ""}[name]
	})
	if errorValue != nil {
		t.Fatalf("a component this tree cannot speak for stopped an unrelated deploy: %v", errorValue)
	}
	if carried["blueclawPayload"].Revision != "payload-7" {
		t.Fatalf("the device must keep the payload it has, got %+v", carried["blueclawPayload"])
	}
}

// admind and capabilityd both embed the generated capability catalog, so a
// release that regenerates the protocol changes the contract they serve. While
// their source paths were kept by hand the catalog was absent from both, their
// revisions did not move, and the guard carried a device forward whose halves
// pinned different protocol hashes.
func TestAGoComponentCoversEverythingItCompiles(t *testing.T) {
	for componentName, mainPackagePath := range map[string]string{
		"admind":      "cmd/internkim-admind",
		"capabilityd": "cmd/internkim-capabilityd",
	} {
		t.Run(componentName, func(t *testing.T) {
			paths := goComponentSourcePaths(repositoryRootForTest(t), mainPackagePath)
			if len(paths) == 0 {
				t.Fatalf("expected %s to report the packages it compiles", componentName)
			}
			for _, required := range []string{"pkg/capabilityprotocol", "internal/capabilities"} {
				if !slices.Contains(paths, required) {
					t.Fatalf("expected %s source paths to cover %s, got %v", componentName, required, paths)
				}
			}
			if entry := componentSourcePaths[componentName]; len(entry) != 1 || entry[0] != mainPackagePath {
				t.Fatalf("expected %s to name only its main package so the revision is derived, got %v", componentName, entry)
			}
		})
	}
}

func repositoryRootForTest(t *testing.T) string {
	t.Helper()
	root := strings.TrimSpace(runCmd("git", "rev-parse", "--show-toplevel"))
	if root == "" {
		t.Skip("release carry test needs a git checkout")
	}
	return root
}
