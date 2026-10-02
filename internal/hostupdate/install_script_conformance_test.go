package hostupdate

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func publishedInstallScript(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	contents, errorValue := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "..", "..", "web", "static", "install.sh"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(contents)
}

func assignedValue(t *testing.T, script string, name string) string {
	t.Helper()
	match := regexp.MustCompile(`(?m)^` + name + `="([^"$]+)"$`).FindStringSubmatch(script)
	if match == nil {
		t.Fatalf("install.sh no longer assigns %s a literal", name)
	}
	return match[1]
}

func TestInstallScriptReadsTheReleasesOfTheRepositoryTheAgentAsks(t *testing.T) {
	if got := assignedValue(t, publishedInstallScript(t), "release_repository"); got != Repository {
		t.Fatalf("install.sh installs from %s and the agent reads releases of %s", got, Repository)
	}
}

func TestInstallScriptRecordsTheChannelWhereTheAgentReadsIt(t *testing.T) {
	if got := assignedValue(t, publishedInstallScript(t), "channel_record_path"); got != ChannelPath {
		t.Fatalf("install.sh records the channel at %s and the agent reads %s", got, ChannelPath)
	}
}

func TestInstallScriptAsksForPackageManagersInTheOrderTheAgentDoes(t *testing.T) {
	match := regexp.MustCompile(`for candidate in ([a-z\- ]+); do`).FindStringSubmatch(publishedInstallScript(t))
	if match == nil {
		t.Fatal("install.sh no longer lists the package managers it looks for")
	}
	commands := []string{}
	for _, manager := range linuxPackageManagers {
		commands = append(commands, manager.Command)
	}
	if got, want := strings.Fields(match[1]), commands; strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("install.sh looks for %v and the agent for %v", got, want)
	}
}
