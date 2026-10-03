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
	command := exec.Command(filepath.Join(repositoryRootPath, messengerPrepareScriptPath), messengerPrintRevisionFlag)
	output, errorValue := command.Output()
	var exitError *exec.ExitError
	if errors.As(errorValue, &exitError) {
		return "", fmt.Errorf("`%s %s` failed: %s",
			messengerPrepareScriptPath, messengerPrintRevisionFlag, strings.TrimSpace(string(exitError.Stderr)))
	}
	if errorValue != nil {
		return "", fmt.Errorf("run `%s %s`: %w", messengerPrepareScriptPath, messengerPrintRevisionFlag, errorValue)
	}
	return strings.TrimSpace(string(output)), nil
}

func requireMessengerRevision(repositoryRootPath string, artifactDirectory string, prepareTarget string) error {
	expected, errorValue := expectedMessengerRevision(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	programs := strings.Join(messengerProgramNames, " and ")
	revisionPath := filepath.Join(repositoryRootPath, artifactDirectory, messengerRevisionFileName)
	rebuild := fmt.Sprintf("rebuild them with `%s --target %s`", messengerPrepareScriptPath, prepareTarget)
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
