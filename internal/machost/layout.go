package machost

import (
	"fmt"
	"path/filepath"
)

// vfkit binds one unix socket per guest port under <runtimeRoot>/vfkit/<instance>/root/,
// and macOS caps sun_path at 104 bytes. The supervisor reports the overflow as a malformed
// flag rather than a long path, so the length is checked while there is still a person
// reading the output.
const (
	vfkitSocketPathBudget            = 104
	vfkitSocketPathBytesAfterRuntime = 68
)

type Layout struct {
	InstallRootPath string
	RuntimeRootPath string
}

func NewLayout(installRootPath string, runtimeRootPath string) Layout {
	return Layout{InstallRootPath: installRootPath, RuntimeRootPath: runtimeRootPath}
}

func (layout Layout) KernelImagePath() string {
	return filepath.Join(layout.InstallRootPath, "vmlinux.bin")
}

func (layout Layout) RootFilesystemImagePath() string {
	return filepath.Join(layout.InstallRootPath, "rootfs.ext4")
}

func (layout Layout) WorkspaceImagePath() string {
	return filepath.Join(layout.InstallRootPath, "workspace.ext4")
}

func (layout Layout) DeliveryPath() string {
	return filepath.Join(layout.InstallRootPath, "delivery")
}

func (layout Layout) DeliveryConfigurationPath() string {
	return filepath.Join(layout.DeliveryPath(), "config")
}

func (layout Layout) DeliveryRuntimePath() string {
	return filepath.Join(layout.DeliveryPath(), "runtime", "current")
}

func (layout Layout) DeliverySkillsPath() string {
	return filepath.Join(layout.DeliveryPath(), "skills")
}

func (layout Layout) RuntimeConfigurationPath() string {
	return filepath.Join(layout.DeliveryConfigurationPath(), "runtime.json")
}

func (layout Layout) PolicyPath() string {
	return filepath.Join(layout.DeliveryConfigurationPath(), "policy.json")
}

func (layout Layout) SupervisorBinaryPath() string {
	return filepath.Join(layout.InstallRootPath, "bin", "blueclaw-supervisor")
}

func (layout Layout) LogDirectoryPath() string {
	return filepath.Join(layout.InstallRootPath, "logs")
}

func (layout Layout) AssertVSockSocketPathFits() error {
	longestSocketPathLength := len(layout.RuntimeRootPath) + vfkitSocketPathBytesAfterRuntime
	if longestSocketPathLength >= vfkitSocketPathBudget {
		return fmt.Errorf(
			"runtime directory %q leaves %d bytes for a vfkit socket path and macOS allows %d; choose a shorter one",
			layout.RuntimeRootPath, longestSocketPathLength, vfkitSocketPathBudget,
		)
	}
	return nil
}
