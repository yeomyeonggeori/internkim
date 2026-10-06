package hostupdate

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func machineWith(command string, files map[string]string) Machine {
	return Machine{
		LookPath: func(name string) (string, error) {
			if name == command {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not found")
		},
		ReadFile: func(path string) ([]byte, error) {
			contents, isKnown := files[path]
			if !isKnown {
				return nil, os.ErrNotExist
			}
			return []byte(contents), nil
		},
	}
}

func TestEachMachineIsUpdatedByTheManagerTheInstallerUses(t *testing.T) {
	for command, method := range map[string]string{"apt-get": "apt", "dnf": "dnf", "pacman": "pacman", "brew": "brew", "zypper": ""} {
		if got := machineWith(command, nil).UpdateMethod(); got != method {
			t.Fatalf("a machine with %s is updated by %q", command, got)
		}
	}
}

func TestTheChannelIsWhatTheInstallRecordedAndNothingElse(t *testing.T) {
	for recorded, want := range map[string]string{"stable\n": ChannelStable, "testing\n": ChannelTesting, "beta\n": ChannelUnrecorded} {
		if got := machineWith("apt-get", map[string]string{ChannelPath: recorded}).Channel(); got != want {
			t.Fatalf("%q reads as %s", recorded, got)
		}
	}
	if got := machineWith("apt-get", nil).Channel(); got != ChannelUnrecorded {
		t.Fatalf("a host whose install recorded nothing reads as %s", got)
	}
	if got := machineWith("brew", nil).Channel(); got != ChannelStable {
		t.Fatalf("a Mac, which takes the tap's only formula, reads as %s", got)
	}
}

func TestTheReleaseListAnswersStableReleasesNewestFirst(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		io.WriteString(responseWriter, `[{"tag_name":"v2026.10.04.000000","prerelease":true},{"tag_name":"v2026.10.02.000000"},{"tag_name":"v2026.10.03.000000","body":" notes "},{"tag_name":"v2026.10.01.000000","draft":true}]`)
	}))
	defer server.Close()
	stable, errorValue := ReleaseSource{APIURL: server.URL}.StableReleases(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	versions := []string{}
	for _, release := range stable {
		versions = append(versions, release.Version)
	}
	if !slices.Equal(versions, []string{"v2026.10.03.000000", "v2026.10.02.000000"}) || stable[0].Notes != "notes" {
		t.Fatalf("stable releases are %v, the newest noting %q", versions, stable[0].Notes)
	}
}

func pendingNote(t *testing.T) string {
	t.Helper()
	notePath := filepath.Join(t.TempDir(), "host-update.json")
	note := Note{Requester: Requester{Email: "member1@example.com", ConversationID: "conversation-1"}, FromVersion: "v1", ToVersion: "v2", StartedAt: time.Now()}
	if errorValue := WriteNote(notePath, note); errorValue != nil {
		t.Fatal(errorValue)
	}
	return notePath
}

func scriptEnding(scriptError error, given *[]string) ScriptRunner {
	return func(arguments []string, output io.Writer) error {
		*given = arguments
		io.WriteString(output, "installing\n")
		return scriptError
	}
}

func TestTheUpdateRunsTheInstallerPinnedToTheStableReleaseAndRecordsItsSuccess(t *testing.T) {
	notePath := pendingNote(t)
	var given []string
	if errorValue := Update(notePath, "v2", scriptEnding(nil, &given), time.Now); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Equal(given, []string{PackagedInstallScriptPath, "host", "--channel", "stable", "--version", "v2"}) {
		t.Fatalf("install.sh is given %v", given)
	}
	note, _, _ := ReadNote(notePath)
	if !note.IsFinished() || !note.Outcome.Succeeded || note.Outcome.OutputTail != "" {
		t.Fatalf("the note reads %+v", note.Outcome)
	}
}

func TestAFailedUpdateRecordsTheErrorAndWhatTheInstallerSaid(t *testing.T) {
	notePath := pendingNote(t)
	var given []string
	if Update(notePath, "v2", scriptEnding(errors.New("exit status 1"), &given), time.Now) == nil {
		t.Fatal("a failed install.sh reported success")
	}
	note, _, _ := ReadNote(notePath)
	if note.Outcome == nil || note.Outcome.Succeeded || !strings.Contains(note.Outcome.Error, "exit status 1") || note.Outcome.OutputTail != "installing" {
		t.Fatalf("the note reads %+v", note.Outcome)
	}
}

func TestTheUpdateUnitIsATransientUnitOfItsOwn(t *testing.T) {
	got := StartArguments("/usr/bin/internkim", "v2")
	want := []string{"--unit=internkim-host-update", "--description=internkim host update to v2", "--collect", "/usr/bin/internkim", "update", "--version", "v2"}
	if !slices.Equal(got, want) {
		t.Fatalf("systemd-run is given %v", got)
	}
}

func TestTheTagOfAPackageVersionDropsTheEpoch(t *testing.T) {
	for version, tag := range map[string]string{"1:0.0.1": "v0.0.1", "0.0.1+37": "v0.0.1+37", "2026.10.01.000000": "v2026.10.01.000000", "": "", "v0.0.1": "v0.0.1"} {
		if got := TagOf(version); got != tag {
			t.Errorf("TagOf(%q) = %q, want %q", version, got, tag)
		}
	}
}

func TestEveryDateVersionIsOlderThanEveryMilestone(t *testing.T) {
	if !IsOlder("v2026.10.01.000000", "v0.0.1") || IsOlder("v0.0.1", "v2026.10.01.000000") {
		t.Fatal("a date version outranked a milestone")
	}
}
