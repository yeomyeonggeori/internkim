package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

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

func copyTrackedTree(sourcePath string, destinationPath string) error {
	trackedPaths, errorValue := trackedFilesUnder(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	if len(trackedPaths) == 0 {
		return fmt.Errorf("git tracks no file under %s", sourcePath)
	}
	for _, relativePath := range trackedPaths {
		source := filepath.Join(sourcePath, relativePath)
		information, errorValue := os.Stat(source)
		if errorValue != nil {
			return errorValue
		}
		if errorValue := copyFile(source, filepath.Join(destinationPath, relativePath), information.Mode().Perm()); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func trackedFilesUnder(directoryPath string) ([]string, error) {
	output, errorValue := exec.Command("git", "-C", directoryPath, "ls-files", "-z").Output()
	if errorValue != nil {
		return nil, fmt.Errorf("listing the files git tracks under %s: %w", directoryPath, errorValue)
	}
	return strings.FieldsFunc(string(output), func(character rune) bool { return character == 0 }), nil
}
