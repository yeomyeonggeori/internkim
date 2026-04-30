package blueclaw

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func EnsureBlueclawBinary(targetPath string, scriptDir string) error {
	return ensureBlueclawCommandBinary(targetPath, scriptDir, "./cmd/blueclaw")
}

func EnsureBlueclawSupervisorBinary(targetPath string, scriptDir string) error {
	return ensureBlueclawCommandBinary(targetPath, scriptDir, "./cmd/blueclaw-supervisor")
}

func ensureBlueclawCommandBinary(targetPath string, scriptDir string, packagePath string) error {
	blueclawDirectory := BlueclawSubmoduleRoot(scriptDir)
	if _, error := os.Stat(filepath.Join(blueclawDirectory, "go.mod")); error != nil {
		return fmt.Errorf("blueclaw submodule missing at %s", blueclawDirectory)
	}

	if error := os.MkdirAll(filepath.Dir(targetPath), 0o755); error != nil {
		return error
	}

	if error := EnsureBlueclawSubmoduleMain(scriptDir); error != nil {
		return error
	}

	buildCommand := exec.Command("go", "build", "-o", targetPath, packagePath)
	buildCommand.Dir = blueclawDirectory
	buildCommand.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")
	output, error := buildCommand.CombinedOutput()
	if error != nil {
		return fmt.Errorf("build %s: %s", packagePath, string(output))
	}

	return os.Chmod(targetPath, 0o755)
}

func EnsureBlueclawSubmoduleMain(scriptDir string) error {
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

	for _, commandArguments := range [][]string{
		{"fetch", "origin", "main"},
		{"checkout", "main"},
		{"pull", "--ff-only", "origin", "main"},
	} {
		if errorValue := runBlueclawGitCommand(blueclawDirectory, commandArguments...); errorValue != nil {
			return errorValue
		}
	}

	return nil
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
