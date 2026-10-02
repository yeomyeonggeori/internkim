package hostupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const (
	PackageName = "internkim"

	ChannelStable     = "stable"
	ChannelTesting    = "testing"
	ChannelUnrecorded = "unrecorded"

	MethodApt    = "apt"
	MethodDnf    = "dnf"
	MethodPacman = "pacman"
	MethodBrew   = "brew"
)

var ChannelPath = filepath.Join(blueclaw.CompanyHostStateRoot, "release-channel")

var PackagedInstallScriptPath = filepath.Join(blueclaw.CompanyPackageHelperRoot, "install.sh")

type packageManager struct {
	Method       string
	Command      string
	VersionQuery []string
}

var linuxPackageManagers = []packageManager{
	{Method: MethodApt, Command: "apt-get", VersionQuery: []string{"dpkg-query", "-W", "-f", "${Version}", PackageName}},
	{Method: MethodDnf, Command: "dnf", VersionQuery: []string{"rpm", "-q", "--qf", "%{VERSION}", PackageName}},
	{Method: MethodPacman, Command: "pacman", VersionQuery: []string{"pacman", "-Q", PackageName}},
}

type Machine struct {
	LookPath     func(string) (string, error)
	Output       func(name string, arguments ...string) ([]byte, error)
	ReadFile     func(string) ([]byte, error)
	RunningBuild string
}

func LocalMachine(runningBuild string) Machine {
	return Machine{
		LookPath: exec.LookPath,
		Output: func(name string, arguments ...string) ([]byte, error) {
			return exec.Command(name, arguments...).Output()
		},
		ReadFile:     os.ReadFile,
		RunningBuild: runningBuild,
	}
}

func (machine Machine) UpdateMethod() string {
	if manager, isFound := machine.linuxPackageManager(); isFound {
		return manager.Method
	}
	if _, errorValue := machine.LookPath("brew"); errorValue == nil {
		return MethodBrew
	}
	return ""
}

func (machine Machine) linuxPackageManager() (packageManager, bool) {
	for _, manager := range linuxPackageManagers {
		if _, errorValue := machine.LookPath(manager.Command); errorValue == nil {
			return manager, true
		}
	}
	return packageManager{}, false
}

func (machine Machine) InstalledVersion() string {
	manager, isFound := machine.linuxPackageManager()
	if !isFound {
		return TagOf(machine.RunningBuild)
	}
	output, errorValue := machine.Output(manager.VersionQuery[0], manager.VersionQuery[1:]...)
	if errorValue != nil {
		return ""
	}
	return TagOf(packageVersionFrom(manager.Method, string(output)))
}

func packageVersionFrom(method string, output string) string {
	version := strings.TrimSpace(output)
	if method != MethodPacman {
		return version
	}
	fields := strings.Fields(version)
	if len(fields) != 2 {
		return ""
	}
	return strings.SplitN(fields[1], "-", 2)[0]
}

func (machine Machine) Channel() string {
	if machine.UpdateMethod() == MethodBrew {
		return ChannelStable
	}
	recorded, errorValue := machine.ReadFile(ChannelPath)
	if errorValue != nil {
		return ChannelUnrecorded
	}
	switch channel := strings.TrimSpace(string(recorded)); channel {
	case ChannelStable, ChannelTesting:
		return channel
	default:
		return ChannelUnrecorded
	}
}
