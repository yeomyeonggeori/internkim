package blueclaw

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func EnsureBlueclawBinary(targetPath string, scriptDir string) error {
	blueclawDirectory := BlueclawSubmoduleRoot(scriptDir)
	if _, error := os.Stat(filepath.Join(blueclawDirectory, "go.mod")); error != nil {
		return fmt.Errorf("blueclaw submodule missing at %s", blueclawDirectory)
	}

	if error := os.MkdirAll(filepath.Dir(targetPath), 0o755); error != nil {
		return error
	}

	buildCommand := exec.Command("go", "build", "-o", targetPath, "./cmd/blueclaw")
	buildCommand.Dir = blueclawDirectory
	buildCommand.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")
	output, error := buildCommand.CombinedOutput()
	if error != nil {
		return fmt.Errorf("build blueclaw: %s", string(output))
	}

	return os.Chmod(targetPath, 0o755)
}
