package hostupdate

import (
	"fmt"
	"os/exec"
	"strings"
)

const UnitName = "internkim-host-update"

type Supervisor struct {
	Run func(name string, arguments ...string) ([]byte, error)
}

func LocalSupervisor() Supervisor {
	return Supervisor{Run: func(name string, arguments ...string) ([]byte, error) {
		return exec.Command(name, arguments...).CombinedOutput()
	}}
}

func (supervisor Supervisor) IsRunning() bool {
	_, errorValue := supervisor.Run("systemctl", "is-active", "--quiet", UnitName+".service")
	return errorValue == nil
}

func (supervisor Supervisor) Start(hostCommandPath string, toVersion string) error {
	output, errorValue := supervisor.Run("systemd-run", StartArguments(hostCommandPath, toVersion)...)
	if errorValue != nil {
		return fmt.Errorf("systemd-run refused the host update: %v: %s", errorValue, strings.TrimSpace(string(output)))
	}
	return nil
}

func StartArguments(hostCommandPath string, toVersion string) []string {
	return []string{"--unit=" + UnitName, "--description=internkim host update to " + toVersion, "--collect", hostCommandPath, "update", "--version", toVersion}
}
