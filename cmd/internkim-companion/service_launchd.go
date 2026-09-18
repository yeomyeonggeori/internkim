package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

type launchdService struct {
	HomeDirectory string
	UserID        string
	Run           commandRunner
}

func (service launchdService) Describe() string {
	return "launchd agent " + serviceLabel + " at " + service.plistPath()
}

func (service launchdService) plistPath() string {
	return filepath.Join(service.HomeDirectory, "Library", "LaunchAgents", serviceLabel+".plist")
}

func (service launchdService) logPath() string {
	return filepath.Join(service.HomeDirectory, "Library", "Logs", "internkim-companion.log")
}

func (service launchdService) domainTarget() string {
	return "gui/" + service.UserID
}

func (service launchdService) Install(executablePath string, runArguments []string) error {
	if errorValue := os.MkdirAll(filepath.Dir(service.plistPath()), 0o755); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(service.logPath()), 0o755); errorValue != nil {
		return errorValue
	}
	document := launchdPlist(executablePath, runArguments, service.logPath(), servicePath(service.HomeDirectory))
	if errorValue := os.WriteFile(service.plistPath(), []byte(document), 0o644); errorValue != nil {
		return errorValue
	}
	_ = service.Run("launchctl", "bootout", service.domainTarget()+"/"+serviceLabel)
	return service.Run("launchctl", "bootstrap", service.domainTarget(), service.plistPath())
}

func (service launchdService) Uninstall() error {
	_ = service.Run("launchctl", "bootout", service.domainTarget()+"/"+serviceLabel)
	if errorValue := os.Remove(service.plistPath()); errorValue != nil && !os.IsNotExist(errorValue) {
		return errorValue
	}
	return nil
}

func (service launchdService) Restart() error {
	return service.Run("launchctl", "kickstart", "-k", service.domainTarget()+"/"+serviceLabel)
}

func (service launchdService) IsRunning() (bool, error) {
	if _, errorValue := os.Stat(service.plistPath()); errorValue != nil {
		return false, nil
	}
	return runCommandQuietly("launchctl", "print", service.domainTarget()+"/"+serviceLabel) == nil, nil
}

func launchdPlist(executablePath string, runArguments []string, logPath string, path string) string {
	arguments := []string{executablePath, "run"}
	arguments = append(arguments, runArguments...)
	var argumentLines strings.Builder
	for _, argument := range arguments {
		argumentLines.WriteString("\t\t<string>" + escapeXML(argument) + "</string>\n")
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + serviceLabel + `</string>
	<key>ProgramArguments</key>
	<array>
` + argumentLines.String() + `	</array>
	<key>EnvironmentVariables</key>
	<dict>
		<key>PATH</key>
		<string>` + escapeXML(path) + `</string>
	</dict>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ThrottleInterval</key>
	<integer>5</integer>
	<key>StandardOutPath</key>
	<string>` + escapeXML(logPath) + `</string>
	<key>StandardErrorPath</key>
	<string>` + escapeXML(logPath) + `</string>
</dict>
</plist>
`
}

func escapeXML(value string) string {
	var escaped strings.Builder
	_ = xml.EscapeText(&escaped, []byte(value))
	return escaped.String()
}
