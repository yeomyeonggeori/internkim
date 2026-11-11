package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type postInstallRun struct {
	exitedCleanly bool
	output        string
	calls         []string
}

func runTheUpgradeBranch(t *testing.T, hasACompany bool, refreshSucceeds bool) postInstallRun {
	t.Helper()
	directory := t.TempDir()
	callsPath := filepath.Join(directory, "calls")
	currentPath := filepath.Join(directory, "current")
	if hasACompany {
		if errorValue := os.Mkdir(currentPath, 0o700); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	refreshExit := map[bool]string{true: "0", false: "1"}[refreshSucceeds]
	stubs := map[string]string{
		"systemctl": `echo "systemctl $*" >> ` + callsPath,
		"refresh":   `echo "refresh" >> ` + callsPath + "\nexit " + refreshExit,
	}
	for name, body := range stubs {
		if errorValue := os.WriteFile(filepath.Join(directory, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	script := "refuse() {\n  echo \"internkim: $1\" >&2\n  exit 1\n}\n" +
		bringTheCompanyBackOnThisRelease(currentPath, filepath.Join(directory, "refresh")) + "\n"
	command := exec.Command("sh", "-c", script)
	command.Env = append(os.Environ(), "PATH="+directory+":"+os.Getenv("PATH"))
	output, errorValue := command.CombinedOutput()
	recorded, _ := os.ReadFile(callsPath)
	return postInstallRun{
		exitedCleanly: errorValue == nil,
		output:        string(output),
		calls:         strings.Split(strings.TrimSpace(string(recorded)), "\n"),
	}
}

func TestAnUpgradeBringsTheConnectedCompanyBackThroughRefresh(t *testing.T) {
	run := runTheUpgradeBranch(t, true, true)
	expected := []string{"systemctl restart " + boxUnitFileName(), "refresh"}
	if !run.exitedCleanly || !slices.Equal(run.calls, expected) {
		t.Fatalf("an upgrade of a connected host ran %v, expected %v: the box daemon restarts onto this release first, because the bundle waits on what it writes, and the refresh then rewrites the rendered files, restarts the bundle and waits for it\n%s", run.calls, expected, run.output)
	}
}

func TestAnUpgradeRestartsTheBoxEvenWhenTheCompanyDoesNotComeBack(t *testing.T) {
	run := runTheUpgradeBranch(t, true, false)
	if len(run.calls) == 0 || run.calls[0] != "systemctl restart "+boxUnitFileName() {
		t.Fatalf("a failed refresh left the box daemon on the last release, so a retry waits on a daemon that never writes what this release's units need: %v", run.calls)
	}
}

func TestAnUpgradeWhoseCompanyDoesNotComeBackFailsAndSaysHowToRetry(t *testing.T) {
	run := runTheUpgradeBranch(t, true, false)
	if run.exitedCleanly {
		t.Fatalf("the upgrade reported success while the company's server did not answer:\n%s", run.output)
	}
	if !strings.Contains(run.output, "refresh'") {
		t.Fatalf("the refusal does not name the command that retries:\n%s", run.output)
	}
}

func TestAnUpgradeOfAnEmptyBoxRestartsTheUnitsAndRefreshesNothing(t *testing.T) {
	run := runTheUpgradeBranch(t, false, true)
	if !run.exitedCleanly || len(run.calls) != 1 || run.calls[0] != "systemctl restart "+restartedUnitFileNames() {
		t.Fatalf("an upgrade with no company ran %v\n%s", run.calls, run.output)
	}
}

func TestThePostInstallRefreshesWithTheProgramThePackageInstalls(t *testing.T) {
	script := maintainerScript(debianPackageFormat, postInstallScript)
	if !strings.Contains(script, bringTheCompanyBackOnThisRelease(blueclaw.CompanyHostCurrentPath, refreshCommand())) {
		t.Fatalf("the post-install does not bring a connected company back:\n%s", script)
	}
	isInstalled := false
	for _, program := range packagedGoPrograms() {
		isInstalled = isInstalled || program.InstalledPath()+" refresh" == refreshCommand()
	}
	if !isInstalled {
		t.Fatalf("the post-install runs %q, which is no program the package installs", refreshCommand())
	}
}
