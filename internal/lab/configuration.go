package lab

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Configuration struct {
	Host           HostConfiguration           `json:"host"`
	VirtualMachine VirtualMachineConfiguration `json:"vm"`
}

type HostConfiguration struct {
	Mode string `json:"mode"`
}

type VirtualMachineConfiguration struct {
	Container           ContainerConfiguration `json:"container"`
	SharedWorkspacePath string                 `json:"sharedWorkspacePath"`
	MountDirectoryPath  string                 `json:"mountDirectoryPath"`
	SSHUsername         string                 `json:"sshUsername"`
	SSHPassword         string                 `json:"sshPassword"`
}

type ContainerConfiguration struct {
	BinaryPath      string `json:"binaryPath"`
	Name            string `json:"name"`
	Image           string `json:"image"`
	KernelImagePath string `json:"kernelImagePath"`
	CPUCount        int    `json:"cpuCount"`
	MemoryMiB       int    `json:"memoryMiB"`
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
	if configuration.VirtualMachine.Container.BinaryPath == "" {
		configuration.VirtualMachine.Container.BinaryPath = "container"
	}
	if configuration.VirtualMachine.Container.Name == "" {
		configuration.VirtualMachine.Container.Name = "internkim-lab"
	}
	if configuration.VirtualMachine.Container.Image == "" {
		configuration.VirtualMachine.Container.Image = "ubuntu:24.04"
	}
	if configuration.VirtualMachine.Container.CPUCount == 0 {
		configuration.VirtualMachine.Container.CPUCount = 6
	}
	if configuration.VirtualMachine.Container.MemoryMiB == 0 {
		configuration.VirtualMachine.Container.MemoryMiB = 8192
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

	return configuration
}

func DefaultConfigurationPath(repositoryRootPath string) string {
	return filepath.Join(repositoryRootPath, "lab", "config.example.json")
}
