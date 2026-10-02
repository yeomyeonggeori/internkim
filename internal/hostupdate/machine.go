package hostupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const (
	ChannelStable     = "stable"
	ChannelTesting    = "testing"
	ChannelUnrecorded = "unrecorded"
	MethodBrew        = "brew"
)

var (
	ChannelPath               = filepath.Join(blueclaw.CompanyHostStateRoot, "release-channel")
	PackagedInstallScriptPath = filepath.Join(blueclaw.CompanyPackageHelperRoot, "install.sh")
)

var updateMethods = []struct{ Command, Method string }{
	{"apt-get", "apt"},
	{"dnf", "dnf"},
	{"pacman", "pacman"},
	{"brew", MethodBrew},
}

type Machine struct {
	LookPath         func(string) (string, error)
	ReadFile         func(string) ([]byte, error)
	InstalledVersion string
}

func LocalMachine(runningBuild string) Machine {
	return Machine{LookPath: exec.LookPath, ReadFile: os.ReadFile, InstalledVersion: TagOf(runningBuild)}
}

func (machine Machine) UpdateMethod() string {
	for _, candidate := range updateMethods {
		if _, errorValue := machine.LookPath(candidate.Command); errorValue == nil {
			return candidate.Method
		}
	}
	return ""
}

func (machine Machine) Channel() string {
	if machine.UpdateMethod() == MethodBrew {
		return ChannelStable
	}
	recorded, _ := machine.ReadFile(ChannelPath)
	switch channel := strings.TrimSpace(string(recorded)); channel {
	case ChannelStable, ChannelTesting:
		return channel
	default:
		return ChannelUnrecorded
	}
}
