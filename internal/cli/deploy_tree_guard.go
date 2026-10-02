package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
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

func runVerifyDeployTree() error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	return checkDeployTree(repositoryRootPath, ReleaseComponentNames())
}
