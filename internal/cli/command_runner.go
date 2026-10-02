package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type updateCommandRunner func(directoryPath string, name string, arguments ...string) error

func runStreamingUpdateCommand(directoryPath string, name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	command.Dir = directoryPath
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	if errorValue := command.Run(); errorValue != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(arguments, " "), errorValue)
	}
	return nil
}
