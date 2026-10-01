package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/releaseset"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type rollbackRequest struct {
	enabled   bool
	releaseID string
}

func parseRollbackRequest(arguments []string) rollbackRequest {
	for index, argument := range arguments {
		if argument == "--rollback" {
			hasReleaseID := index+1 < len(arguments) && !strings.HasPrefix(arguments[index+1], "-")
			if hasReleaseID {
				return rollbackRequest{enabled: true, releaseID: arguments[index+1]}
			}
			return rollbackRequest{enabled: true}
		}
		if strings.HasPrefix(argument, "--rollback=") {
			return rollbackRequest{enabled: true, releaseID: strings.TrimPrefix(argument, "--rollback=")}
		}
	}
	return rollbackRequest{}
}

type rollbackSources struct {
	history         func() (releaseset.ChannelHistory, error)
	manifest        func(manifestURL string) (releaseset.Manifest, error)
	blobIsPublished func(blobPath string) bool
	migrationsAt    func(blueclawRevision string) ([]string, error)
}

type rollbackPlan struct {
	from    releaseset.ChannelHistoryEntry
	to      releaseset.ChannelHistoryEntry
	target  releaseset.Manifest
	changes []string
}

func planRollback(sources rollbackSources, currentReleaseID string, requestedReleaseID string) (rollbackPlan, error) {
	history, errorValue := sources.history()
	if errorValue != nil {
		return rollbackPlan{}, fmt.Errorf("read the %s channel's release history: %w", deployReleaseChannel, errorValue)
	}
	from, to, errorValue := chooseRollbackEntries(history.Entries, currentReleaseID, requestedReleaseID)
	if errorValue != nil {
		return rollbackPlan{}, errorValue
	}
	current, errorValue := sources.manifest(from.ManifestURL)
	if errorValue != nil {
		return rollbackPlan{}, fmt.Errorf("read the manifest of the release the device holds, %s: %w", from.ReleaseID, errorValue)
	}
	target, errorValue := sources.manifest(to.ManifestURL)
	if errorValue != nil {
		return rollbackPlan{}, fmt.Errorf("read the manifest of %s: %w", to.ReleaseID, errorValue)
	}
	if refusals := rollbackRefusals(sources, current, target); len(refusals) > 0 {
		return rollbackPlan{}, fmt.Errorf("rollback refuses to go from %s to %s:\n  %s", from.ReleaseID, to.ReleaseID, strings.Join(refusals, "\n  "))
	}
	return rollbackPlan{from: from, to: to, target: target, changes: describeRollbackChanges(current, target)}, nil
}

func chooseRollbackEntries(entries []releaseset.ChannelHistoryEntry, currentReleaseID string, requestedReleaseID string) (releaseset.ChannelHistoryEntry, releaseset.ChannelHistoryEntry, error) {
	currentIndex := indexOfReleaseEntry(entries, currentReleaseID)
	if currentIndex < 0 {
		return releaseset.ChannelHistoryEntry{}, releaseset.ChannelHistoryEntry{}, fmt.Errorf(
			"the device holds %q, which the %s channel's retained history does not list, so there is nothing to compare a rollback against", currentReleaseID, deployReleaseChannel)
	}
	if requestedReleaseID == "" {
		if currentIndex+1 >= len(entries) {
			return releaseset.ChannelHistoryEntry{}, releaseset.ChannelHistoryEntry{}, fmt.Errorf("no release older than %s is retained on the %s channel", currentReleaseID, deployReleaseChannel)
		}
		return entries[currentIndex], entries[currentIndex+1], nil
	}
	if requestedReleaseID == currentReleaseID {
		return releaseset.ChannelHistoryEntry{}, releaseset.ChannelHistoryEntry{}, fmt.Errorf("the device already holds %s", currentReleaseID)
	}
	requestedIndex := indexOfReleaseEntry(entries, requestedReleaseID)
	if requestedIndex < 0 {
		return releaseset.ChannelHistoryEntry{}, releaseset.ChannelHistoryEntry{}, fmt.Errorf("%s is not among the releases the %s channel retains: %s", requestedReleaseID, deployReleaseChannel, retainedReleaseIDs(entries))
	}
	return entries[currentIndex], entries[requestedIndex], nil
}

func indexOfReleaseEntry(entries []releaseset.ChannelHistoryEntry, releaseID string) int {
	for index, entry := range entries {
		if entry.ReleaseID == releaseID {
			return index
		}
	}
	return -1
}

func retainedReleaseIDs(entries []releaseset.ChannelHistoryEntry) string {
	identifiers := make([]string, 0, len(entries))
	for _, entry := range entries {
		identifiers = append(identifiers, entry.ReleaseID)
	}
	return strings.Join(identifiers, ", ")
}

func rollbackRefusals(sources rollbackSources, current releaseset.Manifest, target releaseset.Manifest) []string {
	refusals := []string{}
	for _, name := range sortedComponentNames(current.Components) {
		if _, carried := target.Components[name]; !carried {
			refusals = append(refusals, fmt.Sprintf("%s is on the device but absent from the older release, which would leave it running ahead of everything else", name))
		}
	}
	for _, name := range sortedComponentNames(target.Components) {
		component := target.Components[name]
		installed, isInstalled := current.Components[name]
		if isInstalled && installed.SHA256 == component.SHA256 {
			continue
		}
		if !sources.blobIsPublished(component.BlobPath) {
			refusals = append(refusals, fmt.Sprintf("the registry no longer holds %s's blob for that release (it was pruned from retention)", name))
		}
	}
	return append(refusals, migrationRefusals(sources, current, target)...)
}

func migrationRefusals(sources rollbackSources, current releaseset.Manifest, target releaseset.Manifest) []string {
	currentPayload, targetPayload := current.Components["blueclawPayload"], target.Components["blueclawPayload"]
	if currentPayload.SHA256 == targetPayload.SHA256 {
		return nil
	}
	currentMigrations, currentError := sources.migrationsAt(currentPayload.Revision)
	targetMigrations, targetError := sources.migrationsAt(targetPayload.Revision)
	if currentError != nil || targetError != nil {
		return []string{fmt.Sprintf("cannot list the database migrations of blueclaw %s and %s to prove the older payload can run against the database (fetch .dependency/blueclaw): %v",
			shortRevision(currentPayload.Revision), shortRevision(targetPayload.Revision), errors.Join(currentError, targetError))}
	}
	unknownToTarget := migrationsMissingFrom(currentMigrations, targetMigrations)
	if len(unknownToTarget) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("the database has been migrated by %s, which the older payload (blueclaw %s) does not know, and migrations only go forward: %s",
		shortRevision(currentPayload.Revision), shortRevision(targetPayload.Revision), strings.Join(unknownToTarget, ", "))}
}

func migrationsMissingFrom(migrations []string, known []string) []string {
	knownNames := map[string]bool{}
	for _, name := range known {
		knownNames[name] = true
	}
	missing := []string{}
	for _, name := range migrations {
		if !knownNames[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

func describeRollbackChanges(current releaseset.Manifest, target releaseset.Manifest) []string {
	changes := []string{}
	for _, name := range sortedComponentNames(target.Components) {
		installed := current.Components[name]
		if installed.SHA256 == target.Components[name].SHA256 {
			continue
		}
		changes = append(changes, describeShippedComponent(name, installed.Revision, target.Components[name].Revision))
	}
	return changes
}

func sortedComponentNames(components map[string]releaseset.Component) []string {
	names := make([]string, 0, len(components))
	for name := range components {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

var (
	readDeviceReleaseStatus  = fetchDeviceReleaseUpdateStatusForTarget
	applyDeployRelease       = applyPublishedRelease
	moveDeployChannelPointer = publishChannelPointer
	rollbackSourcesFor       = publishedRollbackSources
)

func runReleaseRollback(arguments []string, request rollbackRequest, repositoryRootPath string, target commandTarget) error {
	if strings.TrimSpace(commandArgumentValue(arguments, "--components", "")) != "" {
		return errors.New("--rollback reapplies a whole earlier release; it cannot be narrowed with --components")
	}
	status, errorValue := readDeviceReleaseStatus(target)
	if errorValue != nil {
		return fmt.Errorf("read what the device holds: %w", errorValue)
	}
	if status.Current == nil {
		return errors.New("the device reports no current release to roll back from")
	}
	fetchReleaseHistory(repositoryRootPath)
	plan, errorValue := planRollback(rollbackSourcesFor(repositoryRootPath), status.Current.ReleaseID, request.releaseID)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("Rollback: %s -> %s\n", plan.from.ReleaseID, plan.to.ReleaseID)
	for _, change := range plan.changes {
		fmt.Printf("Reverting %s\n", change)
	}
	if hasCommandArgument(arguments, "--plan") {
		fmt.Println("Plan only: nothing was applied.")
		return nil
	}
	revisionOf := func(name string) string { return plan.target.Components[name].Revision }
	if errorValue := applyDeployRelease(target, plan.to.ReleaseID, sortedComponentNames(plan.target.Components), revisionOf); errorValue != nil {
		return errorValue
	}
	if errorValue := moveDeployChannelPointer(repositoryRootPath, deployReleaseChannel, plan.to); errorValue != nil {
		return fmt.Errorf("the device now holds %s, but the %s channel pointer could not be moved back to it, so the next deploy would carry components from %s: %w",
			plan.to.ReleaseID, deployReleaseChannel, plan.from.ReleaseID, errorValue)
	}
	return nil
}

func publishedRollbackSources(repositoryRootPath string) rollbackSources {
	registry := readOnlyReleaseRegistry{baseURL: os.Getenv("INTERNKIM_RELEASE_PUBLIC_BASE_URL")}
	return rollbackSources{
		history: func() (releaseset.ChannelHistory, error) {
			if strings.TrimSpace(registry.baseURL) == "" {
				return releaseset.ChannelHistory{}, errors.New("INTERNKIM_RELEASE_PUBLIC_BASE_URL is required")
			}
			return fetchReleaseChannelHistory(registry, deployReleaseChannel)
		},
		manifest:        fetchReleaseManifestDocument,
		blobIsPublished: func(blobPath string) bool { return alreadyPublished(registry, blobPath) },
		migrationsAt: func(blueclawRevision string) ([]string, error) {
			return blueclawMigrationsAt(filepath.Join(repositoryRootPath, blueclaw.BlueclawSubmodulePath), blueclawRevision)
		},
	}
}

func blueclawMigrationsAt(blueclawPath string, revision string) ([]string, error) {
	output, errorValue := gitOutput(blueclawPath, "ls-tree", "--name-only", revision, "migrations/")
	if errorValue != nil {
		return nil, fmt.Errorf("git ls-tree %s: %s", shortRevision(revision), strings.TrimSpace(output))
	}
	names := []string{}
	for _, line := range strings.Split(output, "\n") {
		if strings.HasSuffix(line, ".sql") {
			names = append(names, filepath.Base(line))
		}
	}
	return names, nil
}

type readOnlyReleaseRegistry struct {
	baseURL string
}

func (registry readOnlyReleaseRegistry) PublicURL(objectKey string) string {
	return strings.TrimRight(strings.TrimSpace(registry.baseURL), "/") + "/" + strings.TrimLeft(objectKey, "/")
}

func (registry readOnlyReleaseRegistry) PutObject(string, []byte, string) error {
	return errors.New("the read-only release registry cannot write")
}

func (registry readOnlyReleaseRegistry) DeleteObject(string) error {
	return errors.New("the read-only release registry cannot delete")
}

func publishChannelPointer(repositoryRootPath string, channel string, entry releaseset.ChannelHistoryEntry) error {
	publisher, errorValue := releasePublisherFromEnvironment(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	return putChannelPointer(publisher, channel, entry)
}

func putChannelPointer(publisher releaseObjectPublisher, channel string, entry releaseset.ChannelHistoryEntry) error {
	pointerDocument, errorValue := json.MarshalIndent(releaseset.StablePointer{
		ReleaseID:   entry.ReleaseID,
		ManifestURL: entry.ManifestURL,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return publisher.PutObject("channels/"+channel+".json", append(pointerDocument, '\n'), "application/json")
}
