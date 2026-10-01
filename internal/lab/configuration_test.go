package lab

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigurationAppliesDefaults(t *testing.T) {
	workspacePath := t.TempDir()
	configurationPath := filepath.Join(workspacePath, "lab.json")
	errorValue := os.WriteFile(configurationPath, []byte(`{
  "vm": {
    "container": {
      "cpuCount": 4
    }
  }
}`), 0o600)
	if errorValue != nil {
		t.Fatalf("expected configuration to be written: %v", errorValue)
	}

	configuration, errorValue := LoadConfiguration(configurationPath)
	if errorValue != nil {
		t.Fatalf("expected configuration to load: %v", errorValue)
	}

	if configuration.Host.Mode != "single-mac" {
		t.Fatalf("expected default host mode, got %q", configuration.Host.Mode)
	}
	if configuration.VirtualMachine.Container.BinaryPath != "container" {
		t.Fatalf("expected default container binary, got %q", configuration.VirtualMachine.Container.BinaryPath)
	}
	if configuration.VirtualMachine.Container.Name != "internkim-lab" {
		t.Fatalf("expected default container name, got %q", configuration.VirtualMachine.Container.Name)
	}
	if configuration.VirtualMachine.Container.Image != "ubuntu:24.04" {
		t.Fatalf("expected default container image, got %q", configuration.VirtualMachine.Container.Image)
	}
	if configuration.VirtualMachine.Container.MemoryMiB != 8192 {
		t.Fatalf("expected default container memory, got %d", configuration.VirtualMachine.Container.MemoryMiB)
	}
	if configuration.VirtualMachine.Container.CPUCount != 4 {
		t.Fatalf("expected configured container cpu count, got %d", configuration.VirtualMachine.Container.CPUCount)
	}
	if configuration.VirtualMachine.MountDirectoryPath != "/mnt/shared" {
		t.Fatalf("expected default mount directory, got %q", configuration.VirtualMachine.MountDirectoryPath)
	}
	if configuration.VirtualMachine.SSHUsername != "admin" {
		t.Fatalf("expected default ssh username, got %q", configuration.VirtualMachine.SSHUsername)
	}
}

func TestDefaultConfigurationPathUsesLabDirectory(t *testing.T) {
	path := DefaultConfigurationPath("/repo")

	if path != "/repo/lab/config.example.json" {
		t.Fatalf("default configuration path = %q", path)
	}
}
