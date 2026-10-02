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

func machineWith(commands map[string]bool, outputs map[string]string, files map[string]string) Machine {
	return Machine{
		LookPath: func(name string) (string, error) {
			if commands[name] {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not found")
		},
		Output: func(name string, arguments ...string) ([]byte, error) {
			output, isKnown := outputs[name]
			if !isKnown {
				return nil, errors.New("not installed")
			}
			return []byte(output), nil
		},
		ReadFile: func(path string) ([]byte, error) {
			contents, isKnown := files[path]
			if !isKnown {
				return nil, os.ErrNotExist
			}
			return []byte(contents), nil
		},
		RunningBuild: "2026.10.01.000000",
	}
}

func TestEachPackageManagerReportsTheInstalledVersionAsATag(t *testing.T) {
	for _, testCase := range []struct {
		command string
		query   string
		answer  string
	}{
		{"apt-get", "dpkg-query", "2026.10.02.090000"},
		{"dnf", "rpm", "2026.10.02.090000"},
		{"pacman", "pacman", "internkim 2026.10.02.090000-1\n"},
	} {
		machine := machineWith(map[string]bool{testCase.command: true}, map[string]string{testCase.query: testCase.answer}, nil)
		if got := machine.InstalledVersion(); got != "v2026.10.02.090000" {
			t.Fatalf("%s reports %q", testCase.command, got)
		}
	}
}

func TestAMacReportsTheBuildItIsRunning(t *testing.T) {
	machine := machineWith(map[string]bool{"brew": true}, nil, nil)
	if machine.UpdateMethod() != MethodBrew || machine.InstalledVersion() != "v2026.10.01.000000" || machine.Channel() != ChannelStable {
		t.Fatalf("a Mac answers %s %s %s", machine.UpdateMethod(), machine.InstalledVersion(), machine.Channel())
	}
}

func TestTheChannelIsWhatTheInstallRecordedAndNothingElse(t *testing.T) {
	for recorded, want := range map[string]string{"stable\n": ChannelStable, "testing\n": ChannelTesting, "beta\n": ChannelUnrecorded} {
		machine := machineWith(map[string]bool{"apt-get": true}, nil, map[string]string{ChannelPath: recorded})
		if got := machine.Channel(); got != want {
			t.Fatalf("%q reads as %s", recorded, got)
		}
	}
	if got := machineWith(map[string]bool{"apt-get": true}, nil, nil).Channel(); got != ChannelUnrecorded {
		t.Fatalf("a host whose install recorded nothing reads as %s", got)
	}
}

func TestTheReleaseListAnswersOnlyStableReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/releases/latest":
			io.WriteString(responseWriter, `{"tag_name":"v3","published_at":"2026-10-02T00:00:00Z","body":"notes three"}`)
		case "/releases":
			io.WriteString(responseWriter, `[{"tag_name":"v4","prerelease":true},{"tag_name":"v3"},{"tag_name":"v2"},{"tag_name":"v1","draft":true}]`)
		default:
			http.NotFound(responseWriter, request)
		}
	}))
	defer server.Close()
	source := ReleaseSource{APIURL: server.URL}
	latest, isFound, errorValue := source.LatestStable(context.Background())
	if errorValue != nil || !isFound || latest.Version != "v3" || latest.Notes != "notes three" {
		t.Fatalf("latest stable is %+v %v %v", latest, isFound, errorValue)
	}
	stable, errorValue := source.StableReleases(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	versions := []string{}
	for _, release := range stable {
		versions = append(versions, release.Version)
	}
	if !slices.Equal(versions, []string{"v3", "v2"}) {
		t.Fatalf("stable releases are %v", versions)
	}
	previous, isFound := NewestOlderThan(stable, "v3")
	if !isFound || previous.Version != "v2" {
		t.Fatalf("the release before v3 is %+v", previous)
	}
}

func TestARepositoryWithNoStableReleaseHasNoLatest(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, isFound, errorValue := ReleaseSource{APIURL: server.URL}.LatestStable(context.Background())
	if errorValue != nil || isFound {
		t.Fatalf("a repository with no stable release answers %v %v", isFound, errorValue)
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

func runTo(notePath string, installed string, scriptError error) Run {
	run := NewRun("v2", machineWith(map[string]bool{"apt-get": true}, map[string]string{"dpkg-query": installed}, nil))
	run.NotePath = notePath
	run.Output = io.Discard
	run.RunInstallScript = func(scriptPath string, arguments []string, output io.Writer) error {
		io.WriteString(output, "installing\n")
		return scriptError
	}
	return run
}

func TestAFinishedUpdateRecordsTheVersionTheHostReached(t *testing.T) {
	notePath := pendingNote(t)
	if errorValue := runTo(notePath, "2", nil).Execute(); errorValue != nil {
		t.Fatal(errorValue)
	}
	note, _, _ := ReadNote(notePath)
	if !note.IsFinished() || !note.Outcome.Succeeded || note.Outcome.InstalledVersion != "v2" {
		t.Fatalf("the note reads %+v", note.Outcome)
	}
}

func TestAFailedUpdateRecordsTheErrorAndTheVersionTheHostStayedOn(t *testing.T) {
	notePath := pendingNote(t)
	errorValue := runTo(notePath, "1", errors.New("exit status 1")).Execute()
	if errorValue == nil {
		t.Fatal("a failed install.sh reported success")
	}
	note, _, _ := ReadNote(notePath)
	if note.Outcome == nil || note.Outcome.Succeeded || note.Outcome.InstalledVersion != "v1" || !strings.Contains(note.Outcome.Error, "exit status 1") || note.Outcome.OutputTail != "installing" {
		t.Fatalf("the note reads %+v", note.Outcome)
	}
}

func TestAnInstallThatLeavesAnotherVersionIsNotASuccess(t *testing.T) {
	notePath := pendingNote(t)
	if runTo(notePath, "1", nil).Execute() == nil {
		t.Fatal("an install that left v1 in place reported success")
	}
}

func TestTheUpdateRunsTheInstallScriptPinnedToTheStableRelease(t *testing.T) {
	if got := InstallScriptArguments("v2"); !slices.Equal(got, []string{"host", "--channel", "stable", "--version", "v2"}) {
		t.Fatalf("install.sh is given %v", got)
	}
}

func TestTheUpdateUnitIsATransientUnitOfItsOwnThatCarriesATestRelease(t *testing.T) {
	supervisor := Supervisor{LookupEnv: func(name string) (string, bool) {
		return "http://192.0.2.1/releases/latest/download", name == installReleaseURL
	}}
	got := supervisor.StartArguments("/usr/bin/internkim", "v2")
	want := []string{
		"--unit=internkim-host-update",
		"--description=internkim host update to v2",
		"--collect",
		"--setenv=INTERNKIM_INSTALL_RELEASE_URL=http://192.0.2.1/releases/latest/download",
		"/usr/bin/internkim", "update", "--version", "v2",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("systemd-run is given %v", got)
	}
}
