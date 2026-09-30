package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var checkDeployTree = refuseUnshippableTree

func refuseUnshippableTree(repositoryRootPath string, componentNames []string) error {
	reasons, errorValue := unshippableTreeReasons(repositoryRootPath, componentNames)
	if errorValue != nil {
		return errorValue
	}
	if len(reasons) == 0 {
		return nil
	}
	return fmt.Errorf("deploy refuses to ship this tree:\n  %s", strings.Join(reasons, "\n  "))
}

func unshippableTreeReasons(repositoryRootPath string, componentNames []string) ([]string, error) {
	blueclawPath := filepath.Join(repositoryRootPath, blueclaw.BlueclawSubmodulePath)
	for _, repositoryPath := range []string{repositoryRootPath, blueclawPath} {
		if output, errorValue := gitOutput(repositoryPath, "fetch", "--quiet", "origin"); errorValue != nil {
			return nil, fmt.Errorf("git fetch origin failed in %s, so the tree cannot be checked against it: %s", repositoryPath, output)
		}
	}
	reasons := uncommittedSourceReasons(repositoryRootPath, blueclawPath, componentNames)
	reasons = append(reasons, historyReasons(repositoryRootPath, "this tree's HEAD", "origin/main")...)
	reasons = append(reasons, historyReasons(blueclawPath, "blueclaw's HEAD", "blueclaw's origin/main")...)
	return append(reasons, blueclawPointerReasons(repositoryRootPath, blueclawPath)...), nil
}

func uncommittedSourceReasons(repositoryRootPath string, blueclawPath string, componentNames []string) []string {
	rootPaths, buildsFromBlueclaw := sourcePathsOutsideBlueclaw(repositoryRootPath, componentNames)
	reasons := []string{}
	if len(rootPaths) > 0 {
		if changed := trackedChanges(repositoryRootPath, rootPaths...); len(changed) > 0 {
			reasons = append(reasons, "uncommitted changes to files a shipped component builds from, which would ship under a commit's name: "+strings.Join(changed, ", "))
		}
	}
	if !buildsFromBlueclaw {
		return reasons
	}
	if changed := trackedChanges(blueclawPath); len(changed) > 0 {
		reasons = append(reasons, "uncommitted changes inside .dependency/blueclaw, which a shipped component builds from: "+strings.Join(changed, ", "))
	}
	return reasons
}

func sourcePathsOutsideBlueclaw(repositoryRootPath string, componentNames []string) ([]string, bool) {
	paths, buildsFromBlueclaw := map[string]bool{}, false
	for _, name := range componentNames {
		for _, path := range releaseComponentSourcePaths(name, repositoryRootPath) {
			if path == blueclaw.BlueclawSubmodulePath {
				buildsFromBlueclaw = true
				continue
			}
			paths[path] = true
		}
	}
	sorted := make([]string, 0, len(paths))
	for path := range paths {
		sorted = append(sorted, path)
	}
	sort.Strings(sorted)
	return sorted, buildsFromBlueclaw
}

func trackedChanges(repositoryPath string, paths ...string) []string {
	arguments := []string{"status", "--porcelain", "--untracked-files=no"}
	if len(paths) > 0 {
		arguments = append(append(arguments, "--"), paths...)
	}
	output, _ := gitOutput(repositoryPath, arguments...)
	changed := []string{}
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) != "" {
			changed = append(changed, strings.TrimSpace(line[3:]))
		}
	}
	return changed
}

func historyReasons(repositoryPath string, headName string, remoteBranch string) []string {
	contains, errorValue := gitIsAncestor(repositoryPath, "origin/main", "HEAD")
	if errorValue != nil {
		return []string{fmt.Sprintf("cannot tell whether %s contains %s: %v", headName, remoteBranch, errorValue)}
	}
	if contains {
		return nil
	}
	return []string{fmt.Sprintf("%s does not contain %s; update the branch from it (git rebase origin/main) before deploying", headName, remoteBranch)}
}

func blueclawPointerReasons(repositoryRootPath string, blueclawPath string) []string {
	recorded := blueclawPointerOf(repositoryRootPath, "HEAD")
	checkedOut, _ := gitOutput(blueclawPath, "rev-parse", "HEAD")
	if recorded != "" && recorded == strings.TrimSpace(checkedOut) {
		return nil
	}
	return []string{fmt.Sprintf(".dependency/blueclaw is checked out at %s but this tree records %s; run git submodule update --init --recursive", shortRevision(checkedOut), shortRevision(recorded))}
}

func blueclawPointerOf(repositoryRootPath string, committish string) string {
	output, errorValue := gitOutput(repositoryRootPath, "rev-parse", committish+":"+blueclaw.BlueclawSubmodulePath)
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

func gitOutput(repositoryPath string, arguments ...string) (string, error) {
	output, errorValue := exec.Command("git", append([]string{"-C", repositoryPath}, arguments...)...).CombinedOutput()
	return string(output), errorValue
}

func gitIsAncestor(repositoryPath string, ancestor string, descendant string) (bool, error) {
	command := exec.Command("git", "-C", repositoryPath, "merge-base", "--is-ancestor", ancestor, descendant)
	errorValue := command.Run()
	if errorValue == nil {
		return true, nil
	}
	if exitError, isExit := errorValue.(*exec.ExitError); isExit && exitError.ExitCode() == 1 {
		return false, nil
	}
	return false, errorValue
}
