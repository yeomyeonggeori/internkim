package hostupdate

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
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

func TestTheAgentAsksForTheManagersThePackageIsPublishedFor(t *testing.T) {
	commands := []string{}
	for _, candidate := range updateMethods {
		commands = append(commands, candidate.Command)
	}
	published := []string{}
	for _, manager := range blueclaw.PackageManagers() {
		published = append(published, string(manager))
	}
	if want := append(published, "brew"); !slices.Equal(commands, want) {
		t.Fatalf("the agent looks for %v and the package is published for %v", commands, want)
	}
}
