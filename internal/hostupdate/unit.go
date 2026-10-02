package hostupdate

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	UnitName            = "internkim-host-update"
	installReleaseURL   = "INTERNKIM_INSTALL_RELEASE_URL"
	hostCommandFileName = "internkim"
)

type Supervisor struct {
	Run       func(name string, arguments ...string) ([]byte, error)
	LookupEnv func(string) (string, bool)
}

func LocalSupervisor() Supervisor {
	return Supervisor{
		Run: func(name string, arguments ...string) ([]byte, error) {
			return exec.Command(name, arguments...).CombinedOutput()
		},
		LookupEnv: os.LookupEnv,
	}
}

func (supervisor Supervisor) IsRunning() bool {
	_, errorValue := supervisor.Run("systemctl", "is-active", "--quiet", UnitName+".service")
	return errorValue == nil
}

func (supervisor Supervisor) Start(hostCommandPath string, toVersion string) error {
	output, errorValue := supervisor.Run("systemd-run", supervisor.StartArguments(hostCommandPath, toVersion)...)
	if errorValue != nil {
		return fmt.Errorf("systemd-run refused the host update: %v: %s", errorValue, strings.TrimSpace(string(output)))
	}
	return nil
}

func (supervisor Supervisor) StartArguments(hostCommandPath string, toVersion string) []string {
	arguments := []string{
		"--unit=" + UnitName,
		"--description=internkim host update to " + toVersion,
		"--collect",
	}
	if releaseURL, isSet := supervisor.LookupEnv(installReleaseURL); isSet {
		arguments = append(arguments, "--setenv="+installReleaseURL+"="+releaseURL)
	}
	return append(arguments, hostCommandPath, "update", "--version", toVersion)
}

func HostCommandPath(binaryRoot string) string {
	return strings.TrimRight(binaryRoot, "/") + "/" + hostCommandFileName
}
