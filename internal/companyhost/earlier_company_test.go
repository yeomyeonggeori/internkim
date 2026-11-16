package companyhost

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func writeSample(t *testing.T, path string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, []byte("an earlier company's\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func entriesOf(t *testing.T, directoryPath string) []string {
	t.Helper()
	entries, errorValue := os.ReadDir(directoryPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func samplePlaces(root string) companyPlaces {
	return companyPlaces{
		StateRootPath:   filepath.Join(root, "state"),
		KeptInStateRoot: "box",
		StateDirectories: []blueclaw.CompanyHostDirectory{
			{Path: filepath.Join(root, "state", "companies"), Mode: 0o700},
			{Path: filepath.Join(root, "state", "media", "bucket"), Mode: 0o750},
		},
		EmptiedPaths:     []string{filepath.Join(root, "workspace"), filepath.Join(root, "database"), filepath.Join(root, "never-made")},
		RemovedPaths:     []string{filepath.Join(root, "configuration", "agent-key"), filepath.Join(root, "configuration", "never-written")},
		DatabaseDataPath: filepath.Join(root, "database"),
	}
}

func TestForgettingACompanyLeavesOnlyWhatTheComputerItselfOwns(t *testing.T) {
	root := t.TempDir()
	places := samplePlaces(root)
	for _, path := range []string{
		"state/companies/earlier/secrets/agent-key", "state/media/bucket/picture", "state/chatd/ledger", "state/buzz-account-links.json",
		"state/box/identity.json", "workspace/private/people/one/notes.md", "workspace/.blueclaw/identity-map.json",
		"database/PG_VERSION", "database/base/1/2", "database.before-take-over-20261113T080015Z/PG_VERSION",
		"configuration/agent-key", "configuration/company-host.env",
	} {
		writeSample(t, filepath.Join(root, path))
	}
	if errorValue := os.Symlink(filepath.Join(root, "state/companies/earlier"), filepath.Join(root, "state/current")); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := places.forget(); errorValue != nil {
		t.Fatal(errorValue)
	}

	if left := entriesOf(t, filepath.Join(root, "state")); !slices.Equal(left, []string{"box", "companies", "media"}) {
		t.Fatalf("the state root holds %v; the computer's own box state stays beside the directories the package ships", left)
	}
	for _, shipped := range []string{"state/companies", "state/media/bucket"} {
		if left := entriesOf(t, filepath.Join(root, shipped)); len(left) != 0 {
			t.Fatalf("%s is made again holding %v; a service that starts on a missing directory fails, and one that finds the earlier company's files serves them", shipped, left)
		}
	}
	if information, errorValue := os.Stat(filepath.Join(root, "state/media/bucket")); errorValue != nil || information.Mode().Perm() != 0o750 {
		t.Fatalf("the bucket directory is %v (%v); it keeps the mode the package gives it", information, errorValue)
	}
	if _, errorValue := os.Stat(filepath.Join(root, "state/box/identity.json")); errorValue != nil {
		t.Fatalf("the box key went with the company: %v", errorValue)
	}
	for _, emptied := range []string{"workspace", "database"} {
		if left := entriesOf(t, filepath.Join(root, emptied)); len(left) != 0 {
			t.Fatalf("%s still holds %v", emptied, left)
		}
	}
	if left := entriesOf(t, root); len(left) != 4 {
		t.Fatalf("beside the emptied directories there is %v; a database an earlier take-over set aside goes too", left)
	}
	if left := entriesOf(t, filepath.Join(root, "configuration")); len(left) != 1 || left[0] != "company-host.env" {
		t.Fatalf("the configuration holds %v; the operator's own file stays and the earlier company's key goes", left)
	}
}

func TestForgettingACompanyTwiceIsTheSameAsOnce(t *testing.T) {
	root := t.TempDir()
	places := samplePlaces(root)
	writeSample(t, filepath.Join(root, "state/box/identity.json"))
	writeSample(t, filepath.Join(root, "workspace/shared/file"))

	for attempt := 0; attempt < 2; attempt++ {
		if errorValue := places.forget(); errorValue != nil {
			t.Fatalf("attempt %d: %v", attempt, errorValue)
		}
	}

	if _, errorValue := os.Stat(filepath.Join(root, "state/box/identity.json")); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func commandsOf(machine *recordedMachine) []string {
	commands := []string{}
	for _, run := range machine.runs {
		commands = append(commands, filepath.Base(run[0]))
	}
	return commands
}

func TestACompanyIsForgottenOnlyAfterItsServicesStopAndItsPeopleAreGone(t *testing.T) {
	root := t.TempDir()
	places := samplePlaces(root)
	writeSample(t, filepath.Join(root, "workspace/private/people/one/notes.md"))
	machine := &recordedMachine{}

	if errorValue := forgetEveryCompanyOn(linuxPlatform{}, places, machine, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}

	if commands := commandsOf(machine); !slices.Equal(commands, []string{"systemctl", "systemctl", "blueclaw-posix-helper"}) {
		t.Fatalf("ran %v; the services stop, then the accounts go, and only then the files", commands)
	}
	if run := machine.runs[2]; !slices.Equal(run[1:], []string{"forget-projected-identities"}) {
		t.Fatalf("the helper was asked %v", run[1:])
	}
	if left := entriesOf(t, filepath.Join(root, "workspace")); len(left) != 0 {
		t.Fatalf("the workspace still holds %v", left)
	}
}

func TestNothingIsRemovedWhenTheServicesWillNotStop(t *testing.T) {
	root := t.TempDir()
	places := samplePlaces(root)
	kept := filepath.Join(root, "workspace/private/people/one/notes.md")
	writeSample(t, kept)
	machine := &recordedMachine{failures: map[string]error{"systemctl": errors.New("Job for blueclaw.service failed")}}

	errorValue := forgetEveryCompanyOn(linuxPlatform{}, places, machine, io.Discard)

	if errorValue == nil || len(machine.runs) != 1 {
		t.Fatalf("got %v after %v; a service that will not stop ends the attempt there", errorValue, machine.runs)
	}
	if _, errorValue := os.Stat(kept); errorValue != nil {
		t.Fatalf("a file went while its company's services still ran: %v", errorValue)
	}
}

func TestNoFileIsRemovedWhenThePeopleCannotBe(t *testing.T) {
	root := t.TempDir()
	places := samplePlaces(root)
	kept := filepath.Join(root, "workspace/private/people/one/notes.md")
	writeSample(t, kept)
	helperPath := linuxPlatform{}.Layout().POSIXHelperPath()
	machine := &recordedMachine{failures: map[string]error{helperPath: errors.New("exit status 1")}}

	errorValue := forgetEveryCompanyOn(linuxPlatform{}, places, machine, io.Discard)

	if errorValue == nil {
		t.Fatal("a helper that refused was taken for one that removed the accounts")
	}
	if _, errorValue := os.Stat(kept); errorValue != nil {
		t.Fatalf("a file went while the account that owns it stayed: %v", errorValue)
	}
}
