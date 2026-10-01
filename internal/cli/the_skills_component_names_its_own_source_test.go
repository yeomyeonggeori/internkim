package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/blueclawworkspace"
)

func TestTheSkillsComponentNamesTheRootsItShips(t *testing.T) {
	repositoryRootPath := repositoryRootForSourcePathTest(t)
	sourcePaths := skillComponentSourcePaths(repositoryRootPath)
	if len(sourcePaths) == 0 {
		t.Fatal("the skills component names no source, so a skills change moves nothing")
	}

	for _, sourcePath := range sourcePaths {
		if _, errorValue := os.Stat(filepath.Join(repositoryRootPath, sourcePath)); errorValue != nil {
			t.Fatalf("a revision path nothing tracks makes the component look changed on every commit: %s: %v", sourcePath, errorValue)
		}
	}

	skillRootPaths, errorValue := blueclawworkspace.SkillRootPaths(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, skillRootPath := range skillRootPaths {
		relativeRootPath, errorValue := filepath.Rel(repositoryRootPath, skillRootPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if !isCoveredBySourcePath(relativeRootPath, sourcePaths) {
			t.Fatalf("the skills component ships %s and would not notice it changing; sources are %v", relativeRootPath, sourcePaths)
		}
	}
}

func isCoveredBySourcePath(relativeRootPath string, sourcePaths []string) bool {
	for _, sourcePath := range sourcePaths {
		if relativeRootPath == sourcePath || strings.HasPrefix(relativeRootPath, sourcePath+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func repositoryRootForSourcePathTest(t *testing.T) string {
	t.Helper()
	workingDirectoryPath, errorValue := os.Getwd()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return filepath.Dir(filepath.Dir(workingDirectoryPath))
}
