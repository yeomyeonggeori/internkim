package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/deployops"
	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
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

Publish a release to the registry, then have the device apply it over ssh.

Options:
  --components <list>  Comma-separated component names to include.
                       Available: ` + strings.Join(ReleaseComponentNames(), ", ") + `
                       Example: --components admind,web
  --release <id>       Override the release ID.
  --channel <name>     Override the release channel (default: stable).
  --fleet <id>         Restrict registry deploy to fleet target ID(s). Repeatable or comma-separated.
  --node <id>          Target a specific node by ID.
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
		"--fleet":      true,
		"--node":       true,
		"--node-id":    true,
		"--host":       true,
		"--device-url": true,
		"--legacy-ssh": true,
		"--all-active": true,
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
		"--fleet":      true,
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

func runRegistryReleaseDeploy(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	registry, errorValue := deployops.LoadRegistry(repositoryRootPath, internkimHomeDir())
	if errorValue != nil {
		return errorValue
	}
	targets, shouldUseRegistry, errorValue := selectRegistryDeployTargets(registry, arguments)
	if errorValue != nil {
		return errorValue
	}
	if !shouldUseRegistry {
		return runDirectReleaseDeploy(arguments)
	}
	return deployReleaseToRegistryTargets(repositoryRootPath, targets, arguments)
}

func selectRegistryDeployTargets(registry deployops.TargetRegistry, arguments []string) ([]deployops.Target, bool, error) {
	fleetIDs := deployFleetIDs(arguments)
	if len(registry.Targets) == 0 {
		return nil, false, nil
	}
	if len(fleetIDs) == 0 && hasExplicitSingleDeployTarget(arguments) {
		return nil, false, nil
	}
	targets, errorValue := deployops.SelectTargets(registry, fleetIDs)
	if errorValue != nil {
		return nil, false, errorValue
	}
	return targets, true, nil
}

func hasExplicitSingleDeployTarget(arguments []string) bool {
	for _, name := range []string{"--host", "--node", "--node-id", "--board", "--sim"} {
		if hasCommandArgument(arguments, name) {
			return true
		}
	}
	return false
}

func deployFleetIDs(arguments []string) []string {
	fleetIDs := []string{}
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		switch {
		case argument == "--fleet" && index+1 < len(arguments):
			fleetIDs = appendFleetIDs(fleetIDs, arguments[index+1])
			index++
		case strings.HasPrefix(argument, "--fleet="):
			fleetIDs = appendFleetIDs(fleetIDs, strings.TrimPrefix(argument, "--fleet="))
		}
	}
	return fleetIDs
}

func appendFleetIDs(fleetIDs []string, value string) []string {
	for _, fleetID := range strings.Split(value, ",") {
		fleetID = strings.TrimSpace(fleetID)
		if fleetID != "" {
			fleetIDs = append(fleetIDs, fleetID)
		}
	}
	return fleetIDs
}

func deployReleaseToRegistryTargets(repositoryRootPath string, targets []deployops.Target, arguments []string) error {
	for _, target := range targets {
		if target.ResolvedKind() != "jetson" {
			return fmt.Errorf("unsupported target kind %q for %s", target.ResolvedKind(), target.ID)
		}
	}
	return deployReleaseToJetsonTargets(repositoryRootPath, targets, arguments)
}

const deployReleaseChannel = "direct"

func deployReleaseToJetsonTargets(repositoryRootPath string, targets []deployops.Target, arguments []string) error {
	if len(targets) == 0 {
		return nil
	}
	selectedComponentNames, errorValue := selectedReleaseComponentNames(arguments)
	if errorValue != nil {
		return errorValue
	}
	releaseID := defaultReleaseID(repositoryRootPath)
	fmt.Printf("Release: %s\n", releaseID)
	if errorValue := publishRelease(repositoryRootPath, releaseID, deployReleaseChannel, selectedComponentNames, 10); errorValue != nil {
		return errorValue
	}
	for _, target := range targets {
		fmt.Printf("Target: %s (%s)\n", target.ID, target.ResolvedKind())
		if errorValue := applyPublishedRelease(commandTargetFromDeployTarget(target), releaseID); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func applyPublishedRelease(target commandTarget, releaseID string) error {
	admin, errorValue := reachDeviceAdmin(target)
	if errorValue != nil {
		return errorValue
	}
	requestDocument, errorValue := json.Marshal(map[string]string{"releaseID": releaseID, "channel": deployReleaseChannel})
	if errorValue != nil {
		return errorValue
	}
	var job blueclawUpdateJobResponse
	if errorValue := admin.ask(http.MethodPost, "/admin/api/updates/apply", requestDocument, &job); errorValue != nil {
		return errorValue
	}
	completedJob, errorValue := waitForReleaseUpdateJob(target, job, releaseID)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("Deploy: %s/%s\n", completedJob.Status, completedJob.Phase)
	return nil
}

func commandTargetFromDeployTarget(target deployops.Target) commandTarget {
	sshUser, sshPassword := resolveSetupSSHCredentials(setup.BoardJetsonOrinNano, "", "")
	return commandTarget{
		mode:           commandTargetModePhysical,
		profile:        target.Profile,
		boardType:      setup.BoardJetsonOrinNano,
		stateDir:       target.StatePath,
		nodeID:         target.NodeID,
		isNodeExplicit: target.NodeArgument != "" || target.NodeID != "",
		deviceURL:      target.AdminURL,
		sshUser:        sshUser,
		sshPassword:    sshPassword,
		sshHostname:    loadState(target.StatePath, "ssh_hostname"),
	}
}

func runDirectReleaseDeploy(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	target := resolveCommandTarget(commandControlArguments(arguments))
	selectedComponentNames, errorValue := selectedReleaseComponentNames(arguments)
	if errorValue != nil {
		return errorValue
	}
	releaseID := defaultReleaseID(repositoryRootPath)
	fmt.Printf("Release: %s\n", releaseID)
	if errorValue := publishRelease(repositoryRootPath, releaseID, deployReleaseChannel, selectedComponentNames, 10); errorValue != nil {
		return errorValue
	}
	return applyPublishedRelease(target, releaseID)
}

func coupleBlueclawWithSkills(selectedComponentNames map[string]bool) map[string]bool {
	if selectedComponentNames["blueclawPayload"] {
		selectedComponentNames["skills"] = true
	}
	if selectedComponentNames["blueclawLLMD"] {
		selectedComponentNames["admind"] = true
		selectedComponentNames["blueclawPayload"] = true
		selectedComponentNames["capabilityd"] = true
		selectedComponentNames["skills"] = true
	}
	return selectedComponentNames
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

func writeDirectReleaseBundleArchive(archivePath string, manifest releaseset.Manifest, blobs []releaseBlob) error {
	file, errorValue := os.Create(archivePath)
	if errorValue != nil {
		return errorValue
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	manifestDocument, errorValue := json.MarshalIndent(manifest, "", "  ")
	if errorValue == nil {
		errorValue = writeDirectReleaseBundleBytes(tarWriter, "manifest.json", append(manifestDocument, '\n'))
	}
	for _, blob := range blobs {
		if errorValue != nil {
			break
		}
		errorValue = writeDirectReleaseBundleFile(tarWriter, "blobs/"+blob.component.Name+".tar.gz", blob.path)
	}
	closeTarError := tarWriter.Close()
	closeGzipError := gzipWriter.Close()
	closeFileError := file.Close()
	for _, candidateError := range []error{errorValue, closeTarError, closeGzipError, closeFileError} {
		if candidateError != nil {
			return candidateError
		}
	}
	return nil
}

func writeDirectReleaseBundleBytes(writer *tar.Writer, name string, document []byte) error {
	header := &tar.Header{Name: name, Mode: 0o600, Size: int64(len(document))}
	if errorValue := writer.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	_, errorValue := writer.Write(document)
	return errorValue
}

func writeDirectReleaseBundleFile(writer *tar.Writer, name string, path string) error {
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		return errorValue
	}
	header, errorValue := tar.FileInfoHeader(information, "")
	if errorValue != nil {
		return errorValue
	}
	header.Name = name
	if errorValue := writer.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	_, errorValue = io.Copy(writer, file)
	return errorValue
}

func waitForReleaseUpdateJob(target commandTarget, job blueclawUpdateJobResponse, expectedReleaseID string) (blueclawUpdateJobResponse, error) {
	lastObservedJob := job
	for attempt := 0; attempt < 240; attempt++ {
		currentJob, errorValue := fetchDeviceReleaseUpdateJob(target, job.JobID)
		if errorValue == nil {
			lastObservedJob = currentJob
			fmt.Printf("Status: %s/%s\n", currentJob.Status, currentJob.Phase)
			switch currentJob.Status {
			case "completed", "already_current":
				return currentJob, nil
			case "failed":
				return currentJob, errors.New(strings.TrimSpace(currentJob.Error))
			}
		} else if releaseUpdateReachedTarget(target, expectedReleaseID) {
			// The job record is gone (an admind-containing release restarts admind and
			// wipes the in-memory job store), but the device's current release already
			// matches the target, so the deploy actually landed.
			fmt.Println("Status: completed/verified (job record cleared by service restart; current release matches target)")
			return blueclawUpdateJobResponse{Status: "completed", Phase: "verified"}, nil
		}
		time.Sleep(1500 * time.Millisecond)
	}
	if releaseUpdateReachedTarget(target, expectedReleaseID) {
		return blueclawUpdateJobResponse{Status: "completed", Phase: "verified"}, nil
	}
	return lastObservedJob, releaseUpdateTimeoutError(target, lastObservedJob)
}

func releaseUpdateTimeoutError(target commandTarget, lastObservedJob blueclawUpdateJobResponse) error {
	lastPhase := strings.TrimSpace(lastObservedJob.Phase)
	if lastPhase == "" {
		lastPhase = "unreported"
	}
	servingReleaseID := "an unreadable release"
	if status, errorValue := fetchDeviceReleaseUpdateStatusForTarget(target); errorValue == nil {
		servingReleaseID = releaseUpdateID(status.Current)
	}
	return fmt.Errorf(
		"release deploy did not finish before timeout at phase %q; the device is still serving %s",
		lastPhase,
		servingReleaseID,
	)
}

func releaseUpdateReachedTarget(target commandTarget, expectedReleaseID string) bool {
	trimmedExpected := strings.TrimSpace(expectedReleaseID)
	if trimmedExpected == "" {
		return false
	}
	status, errorValue := fetchDeviceReleaseUpdateStatusForTarget(target)
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

func releaseManifestComponentNames(manifest releaseset.Manifest) []string {
	componentNames := make([]string, 0, len(manifest.Components))
	for componentName := range manifest.Components {
		componentNames = append(componentNames, componentName)
	}
	sort.Strings(componentNames)
	return componentNames
}
