package blueclaw

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The same nine services, as LaunchDaemons. Everything a plist says about one of
// them is read from CompanyHostServices; nothing here decides what runs.
//
// Three things launchd cannot be told, and what happens to each:
//
//   - Ordering. After=, Requires= and BindsTo= have no counterpart, so the
//     bundle comes up in whatever order launchd pleases and every process has to
//     tolerate its dependencies being absent and retry. `internkim install` polls
//     the same six readiness endpoints it polls on Debian, and on a Mac that
//     polling is the only thing standing where systemd's ordering stood.
//   - EnvironmentFile=. launchd reads no file; EnvironmentVariables is baked into
//     the plist when it is written. So the plists are written by `internkim
//     install` after the company's files exist, and rewritten whenever they
//     change. A plist written before there is a company would carry nothing.
//   - ConditionPathExists=. KeepAlive's PathState is close in effect, and
//     launchd.plist(5) says of it: "Filesystem monitoring mechanisms are
//     inherently race-prone and lossy. This option should be avoided." So the
//     condition is not expressed at all, and what takes its place is that the
//     plists do not exist until the file they would have waited for does.
//
// One difference is a trap rather than a loss. systemd expands ${NAME} inside
// ExecStart; launchd hands ProgramArguments to execve untouched, so a literal
// "${MESSENGER_PLATFORM}" would reach the daemon as its own platform name. Every
// reference is resolved here against the same merged environment, and a name
// nothing sets is an error rather than a daemon started with nonsense.

// CompanyHostLaunchDaemonLabelPrefix is the reverse-DNS namespace the daemons
// are bootstrapped into, which is also how `launchctl print system/…` finds them.
const CompanyHostLaunchDaemonLabelPrefix = "kim.intern."

// CompanyHostLaunchDaemonRoot is where a LaunchDaemon plist belongs. It is not
// Homebrew's prefix: launchd reads the system domain only from here, and a
// daemon that runs as root is a system daemon.
const CompanyHostLaunchDaemonRoot = "/Library/LaunchDaemons"

// CompanyHostLaunchDaemon is one plist `internkim install` writes.
type CompanyHostLaunchDaemon struct {
	ServiceName string
	Label       string
	Contents    string
}

func (daemon CompanyHostLaunchDaemon) FileName() string {
	return daemon.Label + ".plist"
}

func (daemon CompanyHostLaunchDaemon) InstalledPath() string {
	return CompanyHostLaunchDaemonRoot + "/" + daemon.FileName()
}

// CompanyHostLaunchDaemonLabel is what launchctl calls a service.
func CompanyHostLaunchDaemonLabel(serviceName string) string {
	return CompanyHostLaunchDaemonLabelPrefix + serviceName
}

// CompanyHostLaunchDaemons renders the bundle for launchd. environmentFiles maps
// each path a service names to that file's contents; a path that is missing from
// it is treated as an absent file, which is a failure for a required one and
// nothing for an optional one.
func CompanyHostLaunchDaemons(layout CompanyHostLayout, environmentFiles map[string]string) ([]CompanyHostLaunchDaemon, error) {
	daemons := []CompanyHostLaunchDaemon{}
	for _, service := range CompanyHostServices(layout) {
		daemon, errorValue := companyHostLaunchDaemon(layout, service, environmentFiles)
		if errorValue != nil {
			return nil, errorValue
		}
		daemons = append(daemons, daemon)
	}
	return daemons, nil
}

func companyHostLaunchDaemon(layout CompanyHostLayout, service CompanyHostService, environmentFiles map[string]string) (CompanyHostLaunchDaemon, error) {
	environment, errorValue := companyHostServiceEnvironment(service, environmentFiles)
	if errorValue != nil {
		return CompanyHostLaunchDaemon{}, errorValue
	}
	// launchd starts a daemon with /usr/bin:/bin:/usr/sbin:/sbin and nothing
	// else, so psql, redis-cli, git, jq, bun and uv are all absent from a
	// service that shells out to them. systemd's default is no better; it costs
	// nothing on Debian because everything the bundle runs is in /usr/bin.
	if _, isSet := environment["PATH"]; !isSet && layout.SearchPath != "" {
		environment["PATH"] = layout.SearchPath
	}
	command, errorValue := resolveCommandReferences(service, environment)
	if errorValue != nil {
		return CompanyHostLaunchDaemon{}, errorValue
	}
	return CompanyHostLaunchDaemon{
		ServiceName: service.Name,
		Label:       CompanyHostLaunchDaemonLabel(service.Name),
		Contents:    launchDaemonPlist(service, command, environment),
	}, nil
}

// companyHostServiceEnvironment merges the service's sources in the order it
// declares them, so the operator's settings file wins over a rendered default on
// a Mac exactly as it does under systemd.
func companyHostServiceEnvironment(service CompanyHostService, environmentFiles map[string]string) (map[string]string, error) {
	environment := map[string]string{}
	for _, source := range service.Environment {
		if source.FilePath == "" {
			for _, value := range source.Settings {
				environment[value.Name] = value.Value
			}
			continue
		}
		contents, isPresent := environmentFiles[source.FilePath]
		if !isPresent {
			if source.IsOptional {
				continue
			}
			return nil, fmt.Errorf(
				"%s reads %s and that file is not there, so its plist would carry none of what it sets",
				service.Name, source.FilePath)
		}
		for name, value := range parseEnvironmentFile(contents) {
			environment[name] = value
		}
	}
	return environment, nil
}

// parseEnvironmentFile reads systemd's EnvironmentFile format, which is not a
// shell script: a value runs to the end of the line, and a line that is blank or
// starts with # sets nothing.
func parseEnvironmentFile(contents string) map[string]string {
	settings := map[string]string{}
	for _, line := range strings.Split(contents, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, value, isAssignment := strings.Cut(trimmed, "=")
		if !isAssignment {
			continue
		}
		settings[strings.TrimSpace(name)] = value
	}
	return settings
}

func resolveCommandReferences(service CompanyHostService, environment map[string]string) ([]string, error) {
	resolved := []string{}
	for _, argument := range service.Command {
		name, isReference := strings.CutPrefix(argument, "${")
		if !isReference {
			resolved = append(resolved, argument)
			continue
		}
		name = strings.TrimSuffix(name, "}")
		value, isSet := environment[name]
		if !isSet || value == "" {
			return nil, fmt.Errorf(
				"%s is started with ${%s} and nothing in its environment sets it; systemd would have expanded it to an empty argument and launchd would pass the braces through",
				service.Name, name)
		}
		resolved = append(resolved, value)
	}
	return resolved, nil
}

func launchDaemonPlist(service CompanyHostService, command []string, environment map[string]string) string {
	plist := &strings.Builder{}
	plist.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	plist.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	plist.WriteString("<plist version=\"1.0\">\n<dict>\n")
	writePlistString(plist, "Label", CompanyHostLaunchDaemonLabel(service.Name))
	writePlistStringArray(plist, "ProgramArguments", command)
	writePlistDictionary(plist, "EnvironmentVariables", environment)
	if service.Account != "" {
		writePlistString(plist, "UserName", service.Account)
		writePlistString(plist, "GroupName", service.Account)
	}
	if service.WorkingDirectory != "" {
		writePlistString(plist, "WorkingDirectory", service.WorkingDirectory)
	}
	writePlistBoolean(plist, "RunAtLoad", true)
	writeLaunchDaemonLifetime(plist, service)
	logPath := CompanyHostLogPath + "/" + service.Name + ".log"
	writePlistString(plist, "StandardOutPath", logPath)
	writePlistString(plist, "StandardErrorPath", logPath)
	plist.WriteString("</dict>\n</plist>\n")
	return plist.String()
}

// The three lifetimes the declaration distinguishes, in launchd's vocabulary.
// A step that runs once and stays done is LaunchOnlyOnce with no KeepAlive; a
// service that must be up whatever happens is KeepAlive true; everything else is
// restarted only when it exits badly, which is what Restart=on-failure means.
func writeLaunchDaemonLifetime(plist *strings.Builder, service CompanyHostService) {
	switch {
	case service.RunsOnceAndStays:
		writePlistBoolean(plist, "LaunchOnlyOnce", true)
	case service.RestartsEvenOnACleanExit:
		writePlistBoolean(plist, "KeepAlive", true)
	default:
		plist.WriteString("\t<key>KeepAlive</key>\n\t<dict>\n\t\t<key>SuccessfulExit</key>\n\t\t<false/>\n\t</dict>\n")
	}
	if service.RestartAfterSeconds > 0 {
		writePlistInteger(plist, "ThrottleInterval", service.RestartAfterSeconds)
	}
	if service.StopTimeoutSeconds > 0 {
		writePlistInteger(plist, "ExitTimeOut", service.StopTimeoutSeconds)
	}
}

func writePlistString(plist *strings.Builder, key string, value string) {
	plist.WriteString("\t<key>" + escapePlistText(key) + "</key>\n\t<string>" + escapePlistText(value) + "</string>\n")
}

func writePlistInteger(plist *strings.Builder, key string, value int) {
	plist.WriteString("\t<key>" + escapePlistText(key) + "</key>\n\t<integer>" + strconv.Itoa(value) + "</integer>\n")
}

func writePlistBoolean(plist *strings.Builder, key string, value bool) {
	rendered := "<false/>"
	if value {
		rendered = "<true/>"
	}
	plist.WriteString("\t<key>" + escapePlistText(key) + "</key>\n\t" + rendered + "\n")
}

func writePlistStringArray(plist *strings.Builder, key string, values []string) {
	plist.WriteString("\t<key>" + escapePlistText(key) + "</key>\n\t<array>\n")
	for _, value := range values {
		plist.WriteString("\t\t<string>" + escapePlistText(value) + "</string>\n")
	}
	plist.WriteString("\t</array>\n")
}

// The keys are sorted so two installs of the same company produce the same
// plist, which is what lets `internkim install` leave an unchanged daemon alone.
func writePlistDictionary(plist *strings.Builder, key string, values map[string]string) {
	plist.WriteString("\t<key>" + escapePlistText(key) + "</key>\n\t<dict>\n")
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		plist.WriteString("\t\t<key>" + escapePlistText(name) + "</key>\n\t\t<string>" + escapePlistText(values[name]) + "</string>\n")
	}
	plist.WriteString("\t</dict>\n")
}

func escapePlistText(value string) string {
	escaped := &strings.Builder{}
	_ = xml.EscapeText(escaped, []byte(value))
	return escaped.String()
}
