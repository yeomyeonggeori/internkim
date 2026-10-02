package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func runVerifyBlueclawPointer(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	baseBranch := "origin/main"
	if len(arguments) > 0 {
		baseBranch = arguments[0]
	}
	return refuseBackwardsBlueclawPointer(repositoryRootPath, baseBranch)
}

func refuseBackwardsBlueclawPointer(repositoryRootPath string, baseBranch string) error {
	blueclawPath := filepath.Join(repositoryRootPath, blueclaw.BlueclawSubmodulePath)
	gitOutput(blueclawPath, "fetch", "--quiet", "origin", "main")
	head := blueclawPointerOf(repositoryRootPath, "HEAD")
	if head == "" {
		return fmt.Errorf("%s is not a submodule of this commit", blueclaw.BlueclawSubmodulePath)
	}
	if !isAncestorOrFalse(blueclawPath, head, "origin/main") {
		return fmt.Errorf("%s is not on blueclaw's main yet; merge it there first", head)
	}
	base := blueclawPointerOf(repositoryRootPath, baseBranch)
	if base == "" || base == head || !isAncestorOrFalse(blueclawPath, base, "origin/main") {
		return nil
	}
	if isAncestorOrFalse(blueclawPath, base, head) {
		return nil
	}
	return errors.New(strings.Join([]string{
		fmt.Sprintf("%s would move backwards, %s -> %s.", blueclaw.BlueclawSubmodulePath, base, head),
		"A rebase drops a pointer bump whose patch already appeared upstream.",
		"Redo it with: git rebase --reapply-cherry-picks origin/main",
	}, "\n"))
}

func isAncestorOrFalse(repositoryPath string, ancestor string, descendant string) bool {
	isAncestor, _ := gitIsAncestor(repositoryPath, ancestor, descendant)
	return isAncestor
}
