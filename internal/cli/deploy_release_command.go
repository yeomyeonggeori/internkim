package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/releaseset"
)

const releaseUpdateHTTPTimeout = 2 * time.Minute
const releaseUpdateRetryAttempts = 4

var releaseUpdateHTTPClient = &http.Client{
	Timeout: releaseUpdateHTTPTimeout,
	CheckRedirect: func(request *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func deployUsageText() string {
	return `Usage: internkim deploy [OPTIONS]

Publish a release to the registry, then have the device apply it over ssh,
or over its public endpoint with a signed request when ssh is unreachable.

Options:
  --components <list>  Comma-separated component names to ship. Without it the
                       command builds the artifacts, compares the device with
                       this tree, and ships every component that differs
                       together with its protocol partner. It refuses a
                       component the device is ahead on, and ships nothing
                       when the device already matches.
                       Available: ` + strings.Join(ReleaseComponentNames(), ", ") + `
                       Example: --components admind,web
  --release <id>       Override the release ID.
  --channel <name>     Override the release channel (default: stable).
  --node <id>          Target a specific node by ID.
  --plan               Build, select and print what would ship or be refused,
                       reading only the device's current release. Publishes
                       nothing. With --rollback it prints what would go back.
  --rollback [<id>]    Reapply the release before the one the device holds, or
                       the named retained release, instead of building. Skips
                       the device-ahead refusal for this run only. Refuses
                       when the older payload cannot run against the database
                       the newer one migrated, or the registry pruned a blob.
  --legacy-ssh         Copy skills and workspace tools straight over ssh.
  -h, --help           Print this usage and exit.`
}

func validateDeployArguments(arguments []string) error {
	knownFlags := map[string]bool{
		"--help":       true,
		"-h":           true,
		"--components": true,
		"--release":    true,
		"--channel":    true,
		"--node":       true,
		"--node-id":    true,
		"--host":       true,
		"--device-url": true,
		"--legacy-ssh": true,
		"--plan":       true,
		"--rollback":   true,
		"--board":      true,
		"--board-type": true,
		"--sim":        true,
		"--sim-name":   true,
		"--":           true,
	}
	knownValueFlags := map[string]bool{
		"--components": true,
		"--release":    true,
		"--channel":    true,
		"--node":       true,
		"--node-id":    true,
		"--host":       true,
		"--device-url": true,
		"--board":      true,
		"--board-type": true,
		"--sim-name":   true,
	}
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--" {
			break
		}
		if !strings.HasPrefix(argument, "-") {
			continue
		}
		name := argument
		if equalIndex := strings.Index(argument, "="); equalIndex != -1 {
			name = argument[:equalIndex]
		}
		if !knownFlags[name] {
			return fmt.Errorf("unrecognized flag: %s", argument)
		}
		if knownValueFlags[name] && !strings.Contains(argument, "=") {
			index++
		}
	}
	return nil
}

const deployReleaseChannel = "direct"

var (
	readDeviceReleaseComponents = readDeviceComponents
	publishDeployRelease        = publishRelease
)

func applyPublishedRelease(target commandTarget, releaseID string, shippedNames []string, revisionOf func(string) string) error {
	api, errorValue := reachDeviceReleaseAPI(target)
	if errorValue != nil {
		return errorValue
	}
	job, errorValue := api.applyRelease(releaseID, deployReleaseChannel)
	if errorValue != nil {
		return errorValue
	}
	completedJob, errorValue := waitForReleaseUpdateJob(api, job, releaseID)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("Deploy: %s/%s\n", completedJob.Status, completedJob.Phase)
	printSetupDrift(api)
	return verifyDeviceHoldsRelease(api, shippedNames, revisionOf)
}

func verifyDeviceHoldsRelease(api deviceReleaseAPI, shippedNames []string, revisionOf func(string) string) error {
	status, errorValue := api.releaseUpdateStatus()
	if errorValue != nil {
		return fmt.Errorf("the deploy finished but the device's release could not be read back to verify it: %w", errorValue)
	}
	if status.Current == nil {
		return errors.New("the deploy finished but the device reports no current release")
	}
	return verifyDeployedRevisions(shippedNames, revisionOf, status.Current.Components)
}

func printSetupDrift(api deviceReleaseAPI) {
	status, errorValue := api.releaseUpdateStatus()
	if errorValue != nil {
		fmt.Printf("Setup drift: unknown, the release status could not be read: %v\n", errorValue)
		return
	}
	for _, drift := range status.SetupDrift {
		fmt.Printf("Warning: %s\n", drift)
	}
}

func runReleaseDeploy(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	target := resolveCommandTarget(commandControlArguments(arguments))
	if request := parseRollbackRequest(arguments); request.enabled {
		return runReleaseRollback(arguments, request, repositoryRootPath, target)
	}
	selectedComponentNames, errorValue := selectedReleaseComponentNames(arguments)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := checkDeployTree(repositoryRootPath, shippedComponentNames(selectedComponentNames)); errorValue != nil {
		return errorValue
	}
	plan, errorValue := chooseDeployComponents(
		repositoryRootPath,
		selectedComponentNames,
		func() (map[string]releaseset.Component, error) { return readDeviceReleaseComponents(target) },
		func(revision string) bool { return revisionIsKnown(repositoryRootPath, revision) },
		func(older string, newer string) bool { return revisionIsAncestor(repositoryRootPath, older, newer) },
	)
	if errorValue != nil {
		return errorValue
	}
	treeRevision := gitRevision(repositoryRootPath)
	revisionOf := func(name string) string {
		return releaseComponentRevision(name, repositoryRootPath, treeRevision)
	}
	shippedNames := shippedComponentNames(plan.selected)
	if plan.selected == nil || len(plan.selected) > 0 {
		printDeploySelection(shippedNames, plan.held, revisionOf)
	}
	if len(plan.refusals) > 0 {
		return refusalError(plan.refusals)
	}
	if plan.selected != nil && len(plan.selected) == 0 {
		fmt.Println("The device already holds everything this tree builds. Nothing to ship.")
		return nil
	}
	if hasCommandArgument(arguments, "--plan") {
		fmt.Println("Plan only: nothing was published.")
		return nil
	}
	releaseID := defaultReleaseID(repositoryRootPath)
	fmt.Printf("Release: %s\n", releaseID)
	if errorValue := publishDeployRelease(repositoryRootPath, releaseID, deployReleaseChannel, plan.selected, 10); errorValue != nil {
		return errorValue
	}
	return applyPublishedRelease(target, releaseID, shippedNames, revisionOf)
}

func readDeviceComponents(target commandTarget) (map[string]releaseset.Component, error) {
	api, errorValue := reachDeviceReleaseAPI(target)
	if errorValue != nil {
		return nil, errorValue
	}
	status, errorValue := api.releaseUpdateStatus()
	if errorValue != nil {
		return nil, errorValue
	}
	components := map[string]releaseset.Component{}
	if status.Current == nil {
		return components, nil
	}
	for name, brief := range status.Current.Components {
		components[name] = releaseset.Component{Name: name, Revision: brief.Revision}
	}
	return components, nil
}

func shippedComponentNames(selectedComponentNames map[string]bool) []string {
	if selectedComponentNames == nil {
		return ReleaseComponentNames()
	}
	names := make([]string, 0, len(selectedComponentNames))
	for name := range selectedComponentNames {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func selectedReleaseComponentNames(arguments []string) (map[string]bool, error) {
	value := commandArgumentValue(arguments, "--components", "")
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	selectedComponentNames := map[string]bool{}
	for _, componentName := range strings.Split(value, ",") {
		componentName = normalizedReleaseComponentName(strings.TrimSpace(componentName))
		if componentName != "" {
			selectedComponentNames[componentName] = true
		}
	}
	if len(selectedComponentNames) == 0 {
		return nil, errors.New("--components did not name any components")
	}
	return selectedComponentNames, nil
}

func normalizedReleaseComponentName(componentName string) string {
	switch componentName {
	case "adminWeb", "admin-web":
		return "web"
	default:
		return componentName
	}
}

func waitForReleaseUpdateJob(api deviceReleaseAPI, job blueclawUpdateJobResponse, expectedReleaseID string) (blueclawUpdateJobResponse, error) {
	lastObservedJob := job
	for attempt := 0; attempt < 240; attempt++ {
		currentJob, errorValue := api.releaseUpdateJob(job.JobID)
		if errorValue == nil {
			lastObservedJob = currentJob
			fmt.Printf("Status: %s/%s\n", currentJob.Status, currentJob.Phase)
			switch currentJob.Status {
			case "completed", "already_current":
				return currentJob, nil
			case "failed":
				return currentJob, releaseApplyFailure(currentJob)
			}
		} else if releaseUpdateReachedTarget(api, expectedReleaseID) {
			// The job record is gone (an admind-containing release restarts admind and
			// wipes the in-memory job store), but the device's current release already
			// matches the target, so the deploy actually landed.
			fmt.Println("Status: completed/verified (job record cleared by service restart; current release matches target)")
			return blueclawUpdateJobResponse{Status: "completed", Phase: "verified"}, nil
		}
		time.Sleep(1500 * time.Millisecond)
	}
	if releaseUpdateReachedTarget(api, expectedReleaseID) {
		return blueclawUpdateJobResponse{Status: "completed", Phase: "verified"}, nil
	}
	return lastObservedJob, releaseUpdateTimeoutError(api, lastObservedJob)
}

// A release is applied by the admind already on the device, so an install the running
// admind cannot accept fails for every release after it — including one carrying the
// admind that would fix it. The SSH path installs admind outside the release engine,
// which is the only way out once that happens.
func releaseApplyFailure(job blueclawUpdateJobResponse) error {
	detail := strings.TrimSpace(job.Error)
	if !strings.Contains(detail, "manifest mismatch") {
		return errors.New(detail)
	}
	return fmt.Errorf("%s\n  the admind on the device could not accept this install, and it applies every release,"+
		" so no release can replace it — install admind over ssh instead:\n"+
		"    make build && ./internkim setup --only admind --force", detail)
}

func releaseUpdateTimeoutError(api deviceReleaseAPI, lastObservedJob blueclawUpdateJobResponse) error {
	lastPhase := strings.TrimSpace(lastObservedJob.Phase)
	if lastPhase == "" {
		lastPhase = "unreported"
	}
	servingReleaseID := "an unreadable release"
	if status, errorValue := api.releaseUpdateStatus(); errorValue == nil {
		servingReleaseID = releaseUpdateID(status.Current)
	}
	return fmt.Errorf(
		"release deploy did not finish before timeout at phase %q; the device is still serving %s",
		lastPhase,
		servingReleaseID,
	)
}

func releaseUpdateReachedTarget(api deviceReleaseAPI, expectedReleaseID string) bool {
	trimmedExpected := strings.TrimSpace(expectedReleaseID)
	if trimmedExpected == "" {
		return false
	}
	status, errorValue := api.releaseUpdateStatus()
	if errorValue != nil {
		return false
	}
	return releaseUpdateID(status.Current) == trimmedExpected
}

func sendReleaseUpdateRequest(buildRequest func() (*http.Request, error)) (int, []byte, error) {
	var lastError error
	for attempt := 1; attempt <= releaseUpdateRetryAttempts; attempt++ {
		request, buildError := buildRequest()
		if buildError != nil {
			return 0, nil, buildError
		}
		response, errorValue := releaseUpdateHTTPClient.Do(request)
		if errorValue == nil {
			responseBody, _ := io.ReadAll(io.LimitReader(response.Body, recoveryResponseBodyLimitBytes))
			response.Body.Close()
			if response.StatusCode < 500 {
				return response.StatusCode, responseBody, nil
			}
			lastError = fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
		} else {
			lastError = errorValue
		}
		if attempt < releaseUpdateRetryAttempts {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	return 0, nil, lastError
}
