package lab

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Configuration struct {
	Host           HostConfiguration           `json:"host"`
	VirtualMachine VirtualMachineConfiguration `json:"vm"`
	Firecracker    FirecrackerConfiguration    `json:"firecracker"`
}

type HostConfiguration struct {
	Mode      string                 `json:"mode"`
	Companion CompanionConfiguration `json:"companion"`
}

type CompanionConfiguration struct {
	ListenAddress   string `json:"listenAddress"`
	CallbackBaseURL string `json:"callbackBaseURL"`
}

type VirtualMachineConfiguration struct {
	Tart                TartConfiguration       `json:"tart"`
	Mattermost          MattermostConfiguration `json:"mattermost"`
	SharedWorkspacePath string                  `json:"sharedWorkspacePath"`
	MountDirectoryPath  string                  `json:"mountDirectoryPath"`
	SSHUsername         string                  `json:"sshUsername"`
	SSHPassword         string                  `json:"sshPassword"`
}

type TartConfiguration struct {
	BinaryPath    string `json:"binaryPath"`
	Name          string `json:"name"`
	Image         string `json:"image"`
	NestedEnabled bool   `json:"nestedEnabled"`
	CPUCount      int    `json:"cpuCount"`
	MemoryMiB     int    `json:"memoryMiB"`
	DiskGiB       int    `json:"diskGiB"`
}

type MattermostConfiguration struct {
	ListenAddress string `json:"listenAddress"`
}

type FirecrackerConfiguration struct {
	BinaryPath         string `json:"binaryPath"`
	KernelImagePath    string `json:"kernelImagePath"`
	RootfsImagePath    string `json:"rootfsImagePath"`
	WorkspaceImagePath string `json:"workspaceImagePath"`
	VSockCID           uint32 `json:"vsockCID"`
}

func LoadConfiguration(path string) (Configuration, error) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return Configuration{}, errorValue
	}

	var configuration Configuration
	errorValue = json.Unmarshal(document, &configuration)
	if errorValue != nil {
		return Configuration{}, errorValue
	}

	return applyDefaultConfiguration(configuration), nil
}

func applyDefaultConfiguration(configuration Configuration) Configuration {
	if configuration.Host.Mode == "" {
		configuration.Host.Mode = "single-mac"
	}
	if configuration.VirtualMachine.Tart.BinaryPath == "" {
		configuration.VirtualMachine.Tart.BinaryPath = "tart"
	}
	if configuration.VirtualMachine.Tart.Name == "" {
		configuration.VirtualMachine.Tart.Name = "internkim-lab"
	}
	if configuration.VirtualMachine.Tart.Image == "" {
		configuration.VirtualMachine.Tart.Image = "ghcr.io/cirruslabs/ubuntu:latest"
	}
	if configuration.VirtualMachine.Tart.CPUCount == 0 {
		configuration.VirtualMachine.Tart.CPUCount = 6
	}
	if configuration.VirtualMachine.Tart.MemoryMiB == 0 {
		configuration.VirtualMachine.Tart.MemoryMiB = 8192
	}
	if configuration.VirtualMachine.Tart.DiskGiB == 0 {
		configuration.VirtualMachine.Tart.DiskGiB = 80
	}
	if configuration.VirtualMachine.Mattermost.ListenAddress == "" {
		configuration.VirtualMachine.Mattermost.ListenAddress = "127.0.0.1:8065"
	}
	if configuration.VirtualMachine.MountDirectoryPath == "" {
		configuration.VirtualMachine.MountDirectoryPath = "/mnt/shared"
	}
	if configuration.VirtualMachine.SSHUsername == "" {
		configuration.VirtualMachine.SSHUsername = "admin"
	}
	if configuration.VirtualMachine.SSHPassword == "" {
		configuration.VirtualMachine.SSHPassword = "admin"
	}
	if configuration.VirtualMachine.SharedWorkspacePath == "" {
		workingDirectoryPath, errorValue := os.Getwd()
		if errorValue == nil {
			configuration.VirtualMachine.SharedWorkspacePath = workingDirectoryPath
		}
	}
	if configuration.Firecracker.BinaryPath == "" {
		configuration.Firecracker.BinaryPath = "/usr/local/bin/firecracker"
	}

	return configuration
}

func DefaultConfigurationPath(repositoryRootPath string) string {
	return filepath.Join(repositoryRootPath, "lab", "config.example.json")
}
