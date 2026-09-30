package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/releaseset"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func deviceHolding(revisions map[string]string) map[string]releaseset.Component {
	components := map[string]releaseset.Component{}
	for name, revision := range revisions {
		components[name] = releaseset.Component{Revision: revision}
	}
	return components
}

func treeBuilding(revisions map[string]string) func(string) string {
	return func(name string) string { return revisions[name] }
}

func neverAhead(string, string) bool { return false }

func TestSelectionTakesWhatDiffers(t *testing.T) {
	selected, errorValue := selectDeployComponents(
		deviceHolding(map[string]string{"skills": "old", "admind": "same"}),
		treeBuilding(map[string]string{"skills": "new", "admind": "same"}),
		neverAhead,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(selected) != 1 || !selected["skills"] {
		t.Fatalf("selected %v, want only skills", selected)
	}
}

func TestSelectionAddsTheProtocolPartner(t *testing.T) {
	selected, errorValue := selectDeployComponents(
		deviceHolding(map[string]string{"capabilityd": "old", "blueclawPayload": "same"}),
		treeBuilding(map[string]string{"capabilityd": "new", "blueclawPayload": "same"}),
		neverAhead,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !selected["capabilityd"] || !selected["blueclawPayload"] {
		t.Fatalf("selected %v, want capabilityd with blueclawPayload", selected)
	}
}

func TestSelectionRefusesAComponentTheDeviceIsAheadOn(t *testing.T) {
	_, errorValue := selectDeployComponents(
		deviceHolding(map[string]string{"relay": "device", "skills": "old"}),
		treeBuilding(map[string]string{"relay": "tree", "skills": "new"}),
		func(older string, newer string) bool { return older == "tree" && newer == "device" },
	)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "relay") {
		t.Fatalf("a rollback of relay was not refused by name: %v", errorValue)
	}
	if strings.Contains(errorValue.Error(), "skills") {
		t.Fatalf("the refusal names a component that is merely behind: %v", errorValue)
	}
}

func TestSelectionIsEmptyWhenTheDeviceMatches(t *testing.T) {
	revisions := map[string]string{"skills": "same", "web": "same"}
	selected, errorValue := selectDeployComponents(deviceHolding(revisions), treeBuilding(revisions), neverAhead)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if selected == nil || len(selected) != 0 {
		t.Fatalf("selected %v, want an empty set", selected)
	}
}

func TestNarrowedSetIsKeptAndTheProtocolGuardStillRefuses(t *testing.T) {
	restore := prepareReleaseArtifacts
	prepareReleaseArtifacts = func(string) error { return nil }
	defer func() { prepareReleaseArtifacts = restore }()
	narrowed := map[string]bool{"capabilityd": true}
	selected, errorValue := chooseDeployComponents("", narrowed, nil, neverAhead)
	if errorValue != nil || len(selected) != 1 || !selected["capabilityd"] {
		t.Fatalf("narrowed set changed: %v, %v", selected, errorValue)
	}
	errorValue = refuseToSplitTheProtocol(
		map[string]releaseset.Component{"capabilityd": {}},
		map[string]releaseset.Component{"capabilityd": {}, "blueclawPayload": {}},
	)
	if errorValue == nil {
		t.Fatal("a narrowed set that splits the protocol was allowed")
	}
}

func writePayloadManifest(t *testing.T, repositoryRootPath string, revision string) {
	t.Helper()
	directoryPath := filepath.Join(repositoryRootPath, blueclaw.BlueclawPayloadArtifactPath)
	if errorValue := os.MkdirAll(directoryPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	document := `{"blueclawRevision":"` + revision + `"}`
	if errorValue := os.WriteFile(filepath.Join(directoryPath, "manifest.json"), []byte(document), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestStalePayloadArtifactIsRebuiltBeforeItsRevisionIsRead(t *testing.T) {
	repositoryRootPath := t.TempDir()
	writePayloadManifest(t, repositoryRootPath, "three-days-old")
	treeRevision := gitRevision(repositoryRootPath)
	held := map[string]releaseset.Component{}
	for _, name := range ReleaseComponentNames() {
		held[name] = releaseset.Component{Revision: releaseComponentRevision(name, repositoryRootPath, treeRevision)}
	}
	held["blueclawPayload"] = releaseset.Component{Revision: "current"}
	restore := prepareReleaseArtifacts
	prepareReleaseArtifacts = func(rootPath string) error {
		writePayloadManifest(t, rootPath, "current")
		return nil
	}
	defer func() { prepareReleaseArtifacts = restore }()
	selected, errorValue := chooseDeployComponents(
		repositoryRootPath,
		nil,
		func() (map[string]releaseset.Component, error) { return held, nil },
		neverAhead,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if selected["blueclawPayload"] {
		t.Fatal("the payload was compared against the stale artifact, not a rebuilt one")
	}
}

func TestVerificationFailsWhenTheDeviceHoldsAnotherRevision(t *testing.T) {
	deployed := map[string]releaseComponentBrief{"skills": {Revision: "new"}, "web": {Revision: "old"}}
	errorValue := verifyDeployedRevisions(
		[]string{"skills", "web"},
		treeBuilding(map[string]string{"skills": "new", "web": "new"}),
		deployed,
	)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "web") || strings.Contains(errorValue.Error(), "skills") {
		t.Fatalf("verification did not name exactly web: %v", errorValue)
	}
}
