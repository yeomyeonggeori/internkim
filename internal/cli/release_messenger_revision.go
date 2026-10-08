package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const (
	messengerPrepareScriptPath = "tools/prepare-buzz-relay"
	messengerRevisionFileName  = "REVISION"
	messengerPrintRevisionFlag = "--print-revision"
)

var messengerProgramNames = []string{blueclaw.BuzzRelayName, blueclaw.BuzzAdminName}

func expectedMessengerRevision(repositoryRootPath string) (string, error) {
	return expectedPreparedRevision(repositoryRootPath, messengerPrepareScriptPath)
}

func expectedPreparedRevision(repositoryRootPath string, prepareScriptPath string) (string, error) {
	command := exec.Command(filepath.Join(repositoryRootPath, prepareScriptPath), messengerPrintRevisionFlag)
	output, errorValue := command.Output()
	var exitError *exec.ExitError
	if errors.As(errorValue, &exitError) {
		return "", fmt.Errorf("`%s %s` failed: %s",
			prepareScriptPath, messengerPrintRevisionFlag, strings.TrimSpace(string(exitError.Stderr)))
	}
	if errorValue != nil {
		return "", fmt.Errorf("run `%s %s`: %w", prepareScriptPath, messengerPrintRevisionFlag, errorValue)
	}
	return strings.TrimSpace(string(output)), nil
}

func requireMessengerRevision(repositoryRootPath string, artifactDirectory string, prepareTarget string) error {
	return requirePreparedRevision(repositoryRootPath, messengerPrepareScriptPath, strings.Join(messengerProgramNames, " and "), artifactDirectory, prepareTarget)
}

func requirePreparedRevision(repositoryRootPath string, prepareScriptPath string, programs string, artifactDirectory string, prepareTarget string) error {
	expected, errorValue := expectedPreparedRevision(repositoryRootPath, prepareScriptPath)
	if errorValue != nil {
		return errorValue
	}
	revisionPath := filepath.Join(repositoryRootPath, artifactDirectory, messengerRevisionFileName)
	rebuild := fmt.Sprintf("rebuild them with `%s --target %s`", prepareScriptPath, prepareTarget)
	recorded, errorValue := os.ReadFile(revisionPath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return fmt.Errorf("%s in %s record no revision at %s and this tree builds %s; %s",
			programs, artifactDirectory, revisionPath, expected, rebuild)
	}
	if errorValue != nil {
		return errorValue
	}
	found := strings.TrimSpace(string(recorded))
	if found != expected {
		return fmt.Errorf("%s in %s were built at %s and this tree builds %s; %s",
			programs, artifactDirectory, found, expected, rebuild)
	}
	return nil
}
