package blueclaw

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func EnsureBlueclawBinary(targetPath string, scriptDir string) error {
	return ensureBlueclawCommandBinary(targetPath, scriptDir, "./cmd/blueclaw", "linux", "arm64")
}

func EnsureBlueclawSupervisorBinary(targetPath string, scriptDir string) error {
	return ensureBlueclawCommandBinary(targetPath, scriptDir, "./cmd/blueclaw-supervisor", "linux", "arm64")
}

func ensureBlueclawCommandBinary(targetPath string, scriptDir string, packagePath string, operatingSystem string, architecture string) error {
	blueclawDirectory := BlueclawSubmoduleRoot(scriptDir)
	if _, error := os.Stat(filepath.Join(blueclawDirectory, "go.mod")); error != nil {
		return fmt.Errorf("blueclaw submodule missing at %s", blueclawDirectory)
	}

	if error := os.MkdirAll(filepath.Dir(targetPath), 0o755); error != nil {
		return error
	}

	if error := RequireBlueclawSubmoduleOnMain(scriptDir); error != nil {
		return error
	}

	buildCommand := exec.Command("go", "build", "-o", targetPath, packagePath)
	buildCommand.Dir = blueclawDirectory
	buildCommand.Env = append(os.Environ(), "GOOS="+operatingSystem, "GOARCH="+architecture)
	output, error := buildCommand.CombinedOutput()
	if error != nil {
		return fmt.Errorf("build %s: %s", packagePath, string(output))
	}

	return os.Chmod(targetPath, 0o755)
}

func RequireBlueclawSubmoduleOnMain(scriptDir string) error {
	if shouldUseLocalBlueclawSubmodule() {
		return nil
	}

	blueclawDirectory := BlueclawSubmoduleRoot(scriptDir)
	statusCommand := exec.Command("git", "status", "--porcelain")
	statusCommand.Dir = blueclawDirectory
	statusOutput, errorValue := statusCommand.Output()
	if errorValue != nil {
		return fmt.Errorf("check blueclaw submodule status: %w", errorValue)
	}
	if strings.TrimSpace(string(statusOutput)) != "" {
		return fmt.Errorf("blueclaw submodule has local changes; commit or stash them before setup pulls main")
	}

	if errorValue := runBlueclawGitCommand(blueclawDirectory, "fetch", "origin", "main"); errorValue != nil {
		return errorValue
	}
	return requireBlueclawCheckoutIsOnMain(blueclawDirectory)
}

// Moving the checkout here would discard whatever the parent pinned, and a build that
// silently rewinds the submodule is how a pointer bump becomes a backwards deploy.
func requireBlueclawCheckoutIsOnMain(blueclawDirectory string) error {
	checkedOutRevision, errorValue := blueclawGitOutput(blueclawDirectory, "rev-parse", "HEAD")
	if errorValue != nil {
		return errorValue
	}
	if isAncestorOfBlueclawMain(blueclawDirectory, checkedOutRevision) {
		return nil
	}
	return fmt.Errorf(
		"the blueclaw submodule is at %s, which is not on blueclaw's main; merge it there and move the pointer, or set INTERNKIM_BLUECLAW_USE_LOCAL=1 to build what is checked out",
		checkedOutRevision[:12],
	)
}

func isAncestorOfBlueclawMain(blueclawDirectory string, revision string) bool {
	command := exec.Command("git", "merge-base", "--is-ancestor", revision, "origin/main")
	command.Dir = blueclawDirectory
	return command.Run() == nil
}

func blueclawGitOutput(blueclawDirectory string, arguments ...string) (string, error) {
	command := exec.Command("git", arguments...)
	command.Dir = blueclawDirectory
	output, errorValue := command.Output()
	if errorValue != nil {
		return "", fmt.Errorf("blueclaw submodule git %v: %w", arguments, errorValue)
	}
	return strings.TrimSpace(string(output)), nil
}

func shouldUseLocalBlueclawSubmodule() bool {
	return os.Getenv("INTERNKIM_BLUECLAW_USE_LOCAL") == "1"
}

func runBlueclawGitCommand(blueclawDirectory string, arguments ...string) error {
	command := exec.Command("git", arguments...)
	command.Dir = blueclawDirectory
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("git %s: %s", strings.Join(arguments, " "), strings.TrimSpace(string(output)))
	}
	return nil
}
