package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/releaseset"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var prepareReleaseArtifacts = buildReleaseArtifacts

func buildReleaseArtifacts(repositoryRootPath string) error {
	fmt.Println("Building the blueclaw payload")
	if errorValue := runBuildStep(repositoryRootPath, "make", "prepare-blueclaw-payload"); errorValue != nil {
		return errorValue
	}
	if !boardUIIsStale(repositoryRootPath) {
		return nil
	}
	fmt.Println("Building the board UI")
	return runBuildStep(filepath.Join(repositoryRootPath, "web"), "bun", "run", "build:board")
}

func runBuildStep(directoryPath string, name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	command.Dir = directoryPath
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("%s %s failed: %s", name, strings.Join(arguments, " "), strings.TrimSpace(string(output)))
	}
	return nil
}

// The board UI's version is the millisecond it was built, so only the age of
// the artifact against the last commit to web/ can say whether it is current.
func boardUIIsStale(repositoryRootPath string) bool {
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "build", "board-ui", "_app", "version.json"))
	if errorValue != nil {
		return true
	}
	builtAtMilliseconds, errorValue := strconv.ParseInt(boardUIVersion(document), 10, 64)
	if errorValue != nil {
		return true
	}
	committedAtSeconds, parseError := strconv.ParseInt(strings.TrimSpace(runCmd("git", "-C", repositoryRootPath, "log", "-1", "--format=%ct", "--", "web")), 10, 64)
	if parseError != nil {
		return false
	}
	return committedAtSeconds*1000 > builtAtMilliseconds
}

func boardUIVersion(document []byte) string {
	version, errorValue := adminUIVersionOf(document)
	if errorValue != nil {
		return ""
	}
	return version
}

func revisionIsAncestor(repositoryRootPath string, older string, newer string) bool {
	for _, repositoryPath := range []string{repositoryRootPath, filepath.Join(repositoryRootPath, blueclaw.BlueclawSubmodulePath)} {
		if exec.Command("git", "-C", repositoryPath, "merge-base", "--is-ancestor", older, newer).Run() == nil {
			return true
		}
	}
	return false
}

// The component names to ship. nil means the whole set, an empty map means the
// device already holds everything this tree builds.
func chooseDeployComponents(
	repositoryRootPath string,
	narrowedComponentNames map[string]bool,
	readDevice func() (map[string]releaseset.Component, error),
	isAncestor func(older string, newer string) bool,
) (map[string]bool, error) {
	if errorValue := prepareReleaseArtifacts(repositoryRootPath); errorValue != nil {
		return nil, errorValue
	}
	if len(narrowedComponentNames) > 0 {
		return narrowedComponentNames, nil
	}
	held, errorValue := readDevice()
	if errorValue != nil || len(held) == 0 {
		return nil, nil
	}
	treeRevision := gitRevision(repositoryRootPath)
	return selectDeployComponents(held, func(name string) string {
		return releaseComponentRevision(name, repositoryRootPath, treeRevision)
	}, isAncestor)
}

func selectDeployComponents(
	held map[string]releaseset.Component,
	revisionOf func(string) string,
	isAncestor func(older string, newer string) bool,
) (map[string]bool, error) {
	selected := map[string]bool{}
	ahead := []string{}
	for _, name := range ReleaseComponentNames() {
		expected := revisionOf(name)
		carried, isHeld := held[name]
		if expected == "" || (isHeld && carried.Revision == expected) {
			continue
		}
		if isHeld && isAncestor(expected, carried.Revision) {
			ahead = append(ahead, describeComponentAhead(name, carried.Revision, expected))
			continue
		}
		selected[name] = true
	}
	if len(ahead) > 0 {
		sort.Strings(ahead)
		return nil, fmt.Errorf(
			"the device is ahead of this tree, and deploying would roll these back:\n  %s\nupdate this branch from origin/main, or name them in --components to roll back on purpose",
			strings.Join(ahead, "\n  "),
		)
	}
	addProtocolPartners(selected, held)
	return selected, nil
}

func describeComponentAhead(name string, carried string, expected string) string {
	return fmt.Sprintf("%s (the device holds %s, this tree builds %s)", name, shortRevision(carried), shortRevision(expected))
}

func addProtocolPartners(selected map[string]bool, held map[string]releaseset.Component) {
	if len(selected) == 0 {
		return
	}
	speaksProtocol := false
	for _, name := range componentsThatSpeakOneProtocol {
		speaksProtocol = speaksProtocol || selected[name]
	}
	if !speaksProtocol {
		return
	}
	for _, name := range componentsThatSpeakOneProtocol {
		if _, isHeld := held[name]; isHeld {
			selected[name] = true
		}
	}
}

func printDeploySelection(names []string, held map[string]releaseset.Component, revisionOf func(string) string) {
	for _, name := range names {
		fmt.Printf("Shipping %s\n", describeShippedComponent(name, held[name].Revision, revisionOf(name)))
	}
}

func describeShippedComponent(name string, carried string, expected string) string {
	if carried == "" {
		return fmt.Sprintf("%s (new, this tree builds %s)", name, shortRevision(expected))
	}
	return fmt.Sprintf("%s (%s -> %s)", name, shortRevision(carried), shortRevision(expected))
}

func verifyDeployedRevisions(
	shippedNames []string,
	revisionOf func(string) string,
	deployed map[string]releaseComponentBrief,
) error {
	sort.Strings(shippedNames)
	mismatched := []string{}
	for _, name := range shippedNames {
		expected, actual := revisionOf(name), deployed[name].Revision
		fmt.Printf("Verify %s: device %s, tree %s\n", name, shortRevision(actual), shortRevision(expected))
		if actual != expected {
			mismatched = append(mismatched, name)
		}
	}
	if len(mismatched) == 0 {
		return nil
	}
	return fmt.Errorf("the device does not hold this tree's revision of: %s", strings.Join(mismatched, ", "))
}
