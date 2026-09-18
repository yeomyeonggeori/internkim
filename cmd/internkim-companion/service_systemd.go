package main

import (
	"os"
	"path/filepath"
	"strings"
)

const systemdUnitName = "internkim-companion.service"

type systemdUserService struct {
	HomeDirectory string
	Run           commandRunner
}

func (service systemdUserService) Describe() string {
	return "systemd user unit " + systemdUnitName + " at " + service.unitPath()
}

func (service systemdUserService) unitPath() string {
	return filepath.Join(service.HomeDirectory, ".config", "systemd", "user", systemdUnitName)
}

func (service systemdUserService) Install(executablePath string, runArguments []string) error {
	if errorValue := os.MkdirAll(filepath.Dir(service.unitPath()), 0o755); errorValue != nil {
		return errorValue
	}
	document := systemdUnit(executablePath, runArguments, servicePath(service.HomeDirectory))
	if errorValue := os.WriteFile(service.unitPath(), []byte(document), 0o644); errorValue != nil {
		return errorValue
	}
	if errorValue := service.Run("systemctl", "--user", "daemon-reload"); errorValue != nil {
		return errorValue
	}
	return service.Run("systemctl", "--user", "enable", "--now", systemdUnitName)
}

func (service systemdUserService) Uninstall() error {
	_ = service.Run("systemctl", "--user", "disable", "--now", systemdUnitName)
	if errorValue := os.Remove(service.unitPath()); errorValue != nil && !os.IsNotExist(errorValue) {
		return errorValue
	}
	return service.Run("systemctl", "--user", "daemon-reload")
}

func (service systemdUserService) Restart() error {
	return service.Run("systemctl", "--user", "restart", systemdUnitName)
}

func (service systemdUserService) IsRunning() (bool, error) {
	if _, errorValue := os.Stat(service.unitPath()); errorValue != nil {
		return false, nil
	}
	return runCommandQuietly("systemctl", "--user", "is-active", "--quiet", systemdUnitName) == nil, nil
}

func systemdUnit(executablePath string, runArguments []string, path string) string {
	arguments := []string{quoteForSystemd(executablePath), "run"}
	for _, argument := range runArguments {
		arguments = append(arguments, quoteForSystemd(argument))
	}
	return `[Unit]
Description=Intern Kim companion
After=network-online.target

[Service]
ExecStart=` + strings.Join(arguments, " ") + `
Environment=PATH=` + path + `
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`
}

func quoteForSystemd(value string) string {
	if !strings.ContainsAny(value, " \t\"'\\") {
		return value
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}
