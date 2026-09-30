package cli

import (
	"errors"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/releaseset"
)

func releaseEntry(releaseID string) releaseset.ChannelHistoryEntry {
	return releaseset.ChannelHistoryEntry{ReleaseID: releaseID, ManifestURL: "https://registry.example.com/releases/" + releaseID + "/manifest.json"}
}

func releaseManifestOf(components map[string]string) releaseset.Manifest {
	manifest := releaseset.Manifest{Components: map[string]releaseset.Component{}}
	for name, revision := range components {
		manifest.Components[name] = releaseset.Component{Name: name, Revision: revision, SHA256: "sha-" + name + "-" + revision, BlobPath: "blobs/" + name + "-" + revision}
	}
	return manifest
}

type fakeRegistry struct {
	history    []releaseset.ChannelHistoryEntry
	manifests  map[string]releaseset.Manifest
	prunedBlob string
	migrations map[string][]string
}

func (registry fakeRegistry) sources() rollbackSources {
	return rollbackSources{
		history: func() (releaseset.ChannelHistory, error) {
			return releaseset.ChannelHistory{Entries: registry.history}, nil
		},
		manifest: func(manifestURL string) (releaseset.Manifest, error) {
			for releaseID, manifest := range registry.manifests {
				if strings.Contains(manifestURL, "/"+releaseID+"/") {
					return manifest, nil
				}
			}
			return releaseset.Manifest{}, errors.New("no such manifest")
		},
		blobIsPublished: func(blobPath string) bool { return blobPath != registry.prunedBlob },
		migrationsAt: func(revision string) ([]string, error) {
			names, isKnown := registry.migrations[revision]
			if !isKnown {
				return nil, errors.New("unknown revision " + revision)
			}
			return names, nil
		},
	}
}

func threeReleases() fakeRegistry {
	return fakeRegistry{
		history: []releaseset.ChannelHistoryEntry{releaseEntry("r3"), releaseEntry("r2"), releaseEntry("r1")},
		manifests: map[string]releaseset.Manifest{
			"r3": releaseManifestOf(map[string]string{"admind": "a3", "web": "w3", "blueclawPayload": "p3"}),
			"r2": releaseManifestOf(map[string]string{"admind": "a2", "web": "w2", "blueclawPayload": "p2"}),
			"r1": releaseManifestOf(map[string]string{"admind": "a1", "web": "w1", "blueclawPayload": "p1"}),
		},
		migrations: map[string][]string{"p1": {"001.sql"}, "p2": {"001.sql", "002.sql"}, "p3": {"001.sql", "002.sql"}},
	}
}

func TestRollbackGoesBackToTheReleaseBeforeTheCurrentOne(t *testing.T) {
	plan, errorValue := planRollback(threeReleases().sources(), "r3", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if plan.from.ReleaseID != "r3" || plan.to.ReleaseID != "r2" {
		t.Fatalf("rollback goes %s -> %s", plan.from.ReleaseID, plan.to.ReleaseID)
	}
	if strings.Join(plan.changes, "|") != "admind (a3 -> a2)|blueclawPayload (p3 -> p2)|web (w3 -> w2)" {
		t.Fatalf("changes = %v", plan.changes)
	}
}

func TestRollbackFromAnEarlierReleaseStepsBackFromThere(t *testing.T) {
	registry := threeReleases()
	registry.migrations["p1"] = []string{"001.sql", "002.sql"}
	plan, errorValue := planRollback(registry.sources(), "r2", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if plan.to.ReleaseID != "r1" {
		t.Fatalf("rolled back to %s", plan.to.ReleaseID)
	}
}

func TestRollbackToANamedReleaseSkipsTheOnesBetween(t *testing.T) {
	registry := threeReleases()
	registry.migrations["p1"] = []string{"001.sql", "002.sql"}
	plan, errorValue := planRollback(registry.sources(), "r3", "r1")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if plan.to.ReleaseID != "r1" {
		t.Fatalf("rolled back to %s", plan.to.ReleaseID)
	}
}

func TestRollbackRefusesWhatItCannotSelect(t *testing.T) {
	registry := threeReleases()
	testCases := map[string]struct{ current, requested, message string }{
		"nothing older":       {"r1", "", "no release older than r1"},
		"an unknown release":  {"r3", "r0", "not among the releases"},
		"the current release": {"r3", "r3", "already holds r3"},
		"an unlisted device":  {"r9", "", "does not list"},
	}
	for name, testCase := range testCases {
		_, errorValue := planRollback(registry.sources(), testCase.current, testCase.requested)
		if errorValue == nil || !strings.Contains(errorValue.Error(), testCase.message) {
			t.Errorf("%s: error = %v, want %q", name, errorValue, testCase.message)
		}
	}
}

func TestRollbackRefusesAnOlderPayloadThatDoesNotKnowAMigrationTheDatabaseHas(t *testing.T) {
	_, errorValue := planRollback(threeReleases().sources(), "r2", "r1")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "migrations only go forward") || !strings.Contains(errorValue.Error(), "002.sql") {
		t.Fatalf("a rollback past a migration was allowed: %v", errorValue)
	}
}

func TestRollbackRefusesWhenTheMigrationsCannotBeListed(t *testing.T) {
	registry := threeReleases()
	delete(registry.migrations, "p2")
	_, errorValue := planRollback(registry.sources(), "r3", "")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "cannot list the database migrations") {
		t.Fatalf("an unprovable rollback was allowed: %v", errorValue)
	}
}

func TestRollbackRefusesWhenTheRegistryPrunedABlob(t *testing.T) {
	registry := threeReleases()
	registry.prunedBlob = "blobs/web-w2"
	_, errorValue := planRollback(registry.sources(), "r3", "")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "no longer holds web") {
		t.Fatalf("a rollback to a pruned blob was allowed: %v", errorValue)
	}
}

func TestRollbackIgnoresABlobTheDeviceAlreadyRuns(t *testing.T) {
	registry := threeReleases()
	registry.manifests["r2"].Components["web"] = registry.manifests["r3"].Components["web"]
	registry.prunedBlob = registry.manifests["r3"].Components["web"].BlobPath
	if _, errorValue := planRollback(registry.sources(), "r3", ""); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestRollbackRefusesAReleaseThatLacksAComponentTheDeviceRuns(t *testing.T) {
	registry := threeReleases()
	registry.manifests["r3"].Components["relay"] = releaseset.Component{Name: "relay", Revision: "x", SHA256: "sha-relay"}
	_, errorValue := planRollback(registry.sources(), "r3", "")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "relay is on the device but absent") {
		t.Fatalf("a rollback that strands a component was allowed: %v", errorValue)
	}
}

func TestRollbackRequestParsing(t *testing.T) {
	testCases := []struct {
		arguments []string
		expected  rollbackRequest
	}{
		{[]string{"--sim"}, rollbackRequest{}},
		{[]string{"--rollback"}, rollbackRequest{enabled: true}},
		{[]string{"--rollback", "--plan"}, rollbackRequest{enabled: true}},
		{[]string{"--rollback", "r2", "--plan"}, rollbackRequest{enabled: true, releaseID: "r2"}},
		{[]string{"--rollback=r2"}, rollbackRequest{enabled: true, releaseID: "r2"}},
	}
	for _, testCase := range testCases {
		if actual := parseRollbackRequest(testCase.arguments); actual != testCase.expected {
			t.Errorf("parseRollbackRequest(%v) = %+v, want %+v", testCase.arguments, actual, testCase.expected)
		}
	}
	if errorValue := validateDeployArguments([]string{"--rollback", "r2", "--plan"}); errorValue != nil {
		t.Fatal(errorValue)
	}
}

type rollbackRecorder struct {
	appliedReleaseID string
	appliedNames     []string
	verifiedRevision string
	pointerMovedTo   string
}

func stubRollbackEffects(t *testing.T, registry fakeRegistry, currentReleaseID string) *rollbackRecorder {
	t.Helper()
	stubDeployEffects(t)
	recorder := &rollbackRecorder{}
	restoreStatus, restoreSources, restoreApply, restorePointer := readDeviceReleaseStatus, rollbackSourcesFor, applyDeployRelease, moveDeployChannelPointer
	readDeviceReleaseStatus = func(commandTarget) (releaseUpdateStatusResponse, error) {
		return releaseUpdateStatusResponse{Current: &releaseUpdateSummary{ReleaseID: currentReleaseID}}, nil
	}
	rollbackSourcesFor = func(string) rollbackSources { return registry.sources() }
	applyDeployRelease = func(_ commandTarget, releaseID string, names []string, revisionOf func(string) string) error {
		recorder.appliedReleaseID, recorder.appliedNames, recorder.verifiedRevision = releaseID, names, revisionOf("web")
		return nil
	}
	moveDeployChannelPointer = func(_ string, _ string, entry releaseset.ChannelHistoryEntry) error {
		recorder.pointerMovedTo = entry.ReleaseID
		return nil
	}
	t.Cleanup(func() {
		readDeviceReleaseStatus, rollbackSourcesFor, applyDeployRelease, moveDeployChannelPointer = restoreStatus, restoreSources, restoreApply, restorePointer
	})
	return recorder
}

func TestRollbackReappliesTheEarlierReleaseAndVerifiesItsRevisions(t *testing.T) {
	recorder := stubRollbackEffects(t, threeReleases(), "r3")
	checkDeployTree = func(string, []string) error {
		t.Fatal("a rollback ships no tree, so it has no tree to check")
		return nil
	}
	prepareReleaseArtifacts = func(string) error {
		t.Fatal("a rollback built something")
		return nil
	}
	if errorValue := runReleaseDeploy([]string{"--sim", "--rollback"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if recorder.appliedReleaseID != "r2" || strings.Join(recorder.appliedNames, ",") != "admind,blueclawPayload,web" {
		t.Fatalf("applied %s %v", recorder.appliedReleaseID, recorder.appliedNames)
	}
	if recorder.verifiedRevision != "w2" {
		t.Fatalf("the device was verified against %q, not the older release's revision", recorder.verifiedRevision)
	}
	if recorder.pointerMovedTo != "r2" {
		t.Fatalf("the channel pointer was left at %q", recorder.pointerMovedTo)
	}
}

func TestRollbackPlanAppliesNothing(t *testing.T) {
	recorder := stubRollbackEffects(t, threeReleases(), "r3")
	if errorValue := runReleaseDeploy([]string{"--sim", "--rollback", "--plan"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if recorder.appliedReleaseID != "" || recorder.pointerMovedTo != "" {
		t.Fatalf("--plan applied %q and moved the pointer to %q", recorder.appliedReleaseID, recorder.pointerMovedTo)
	}
}

func TestRollbackRefusalAppliesNothing(t *testing.T) {
	recorder := stubRollbackEffects(t, threeReleases(), "r2")
	if errorValue := runReleaseDeploy([]string{"--sim", "--rollback", "r1"}); errorValue == nil {
		t.Fatal("a rollback past a migration ran")
	}
	if recorder.appliedReleaseID != "" {
		t.Fatalf("applied %q despite the refusal", recorder.appliedReleaseID)
	}
}

func TestRollbackCannotBeNarrowed(t *testing.T) {
	recorder := stubRollbackEffects(t, threeReleases(), "r3")
	if errorValue := runReleaseDeploy([]string{"--sim", "--rollback", "--components", "web"}); errorValue == nil || recorder.appliedReleaseID != "" {
		t.Fatalf("a narrowed rollback ran: %v", errorValue)
	}
}

func TestOnlyRollbackBypassesTheDeviceAheadRefusal(t *testing.T) {
	recorder := stubRollbackEffects(t, threeReleases(), "r3")
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	treeRevision := gitRevision(repositoryRootPath)
	held := map[string]releaseset.Component{}
	for _, name := range ReleaseComponentNames() {
		held[name] = releaseset.Component{Revision: releaseComponentRevision(name, repositoryRootPath, treeRevision)}
	}
	held["fonts"] = releaseset.Component{Revision: treeRevision}
	if releaseComponentRevision("fonts", repositoryRootPath, treeRevision) == treeRevision {
		t.Skip("assets/fonts changed in the newest commit, so the device cannot be ahead of it")
	}
	restoreRead, restorePublish := readDeviceReleaseComponents, publishDeployRelease
	readDeviceReleaseComponents = func(commandTarget) (map[string]releaseset.Component, error) { return held, nil }
	publishDeployRelease = func(string, string, string, map[string]bool, int) error {
		t.Fatal("a deploy published over a device that is ahead")
		return nil
	}
	t.Cleanup(func() { readDeviceReleaseComponents, publishDeployRelease = restoreRead, restorePublish })

	refuse := func() {
		if errorValue := runReleaseDeploy([]string{"--sim"}); errorValue == nil || !strings.Contains(errorValue.Error(), "the device is ahead") {
			t.Fatalf("the device-ahead refusal did not hold: %v", errorValue)
		}
	}
	refuse()
	if errorValue := runReleaseDeploy([]string{"--sim", "--rollback"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if recorder.appliedReleaseID != "r2" {
		t.Fatalf("the rollback applied %q", recorder.appliedReleaseID)
	}
	refuse()
}

func TestRollbackNamesTheMigrationsFromGit(t *testing.T) {
	tree := newShippableTree(t)
	commitFile(t, tree.blueclaw, "migrations/001_first.sql", "select 1;")
	older := gitInDirectory(t, tree.blueclaw, "rev-parse", "HEAD")
	commitFile(t, tree.blueclaw, "migrations/002_second.sql", "select 2;")
	newer := gitInDirectory(t, tree.blueclaw, "rev-parse", "HEAD")
	olderMigrations, errorValue := blueclawMigrationsAt(tree.blueclaw, older)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	newerMigrations, errorValue := blueclawMigrationsAt(tree.blueclaw, newer)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Join(olderMigrations, ",") != "001_first.sql" || strings.Join(newerMigrations, ",") != "001_first.sql,002_second.sql" {
		t.Fatalf("migrations = %v and %v", olderMigrations, newerMigrations)
	}
	if _, errorValue := blueclawMigrationsAt(tree.blueclaw, "0000000000000000000000000000000000000000"); errorValue == nil {
		t.Fatal("a revision the checkout does not have was listed")
	}
}
