package companyhost

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// launchdMachine answers `launchctl print` for the labels it holds loaded and
// refuses it for the rest, the way launchd does.
type launchdMachine struct {
	*recordedMachine
	loaded map[string]string
}

func (machine launchdMachine) Output(name string, arguments []string) (string, error) {
	if name != "launchctl" || arguments[0] != "print" {
		return machine.recordedMachine.Output(name, arguments)
	}
	machine.runs = append(machine.runs, append([]string{name}, arguments...))
	printed, isLoaded := machine.loaded[arguments[1]]
	if !isLoaded {
		return "", errors.New("Could not find service in domain for system")
	}
	return printed, nil
}

func actionsOf(machine *recordedMachine) []string {
	actions := []string{}
	for _, run := range machine.runs {
		if run[0] == "launchctl" && run[1] == "print" {
			continue
		}
		actions = append(actions, strings.Join(run, " "))
	}
	return actions
}

func TestEachVerbIsOneSystemctlCommandOverTheNamedUnits(t *testing.T) {
	names := []string{blueclaw.BoxServiceName, blueclaw.CompanyHostTakeOverServiceName}
	verbs := map[string]func(Machine) error{
		"enable":  func(machine Machine) error { return linuxPlatform{}.EnableServices(machine, names) },
		"disable": func(machine Machine) error { return linuxPlatform{}.DisableServices(machine, names) },
		"start":   func(machine Machine) error { return linuxPlatform{}.StartServices(machine, names, io.Discard) },
		"stop":    func(machine Machine) error { return linuxPlatform{}.StopServices(machine, names, io.Discard) },
		"restart": func(machine Machine) error { return linuxPlatform{}.RestartServices(machine, names, io.Discard) },
	}
	for verb, act := range verbs {
		machine := &recordedMachine{}
		if errorValue := act(machine); errorValue != nil {
			t.Fatal(errorValue)
		}
		want := []string{"systemctl " + verb + " internkim-box.service internkim-take-over.service"}
		if actions := actionsOf(machine); !slices.Equal(actions, want) {
			t.Fatalf("%s ran %v, want %v", verb, actions, want)
		}
	}
}

func TestSystemdsWordsBecomeTheThreeStatesACallerActsOn(t *testing.T) {
	for activeState, want := range map[string]ServiceState{
		"active":       ServiceRunning,
		"activating":   ServiceRunning,
		"deactivating": ServiceRunning,
		"failed":       ServiceFailed,
		"inactive":     ServiceStopped,
		"":             ServiceStopped,
	} {
		machine := &recordedMachine{answers: map[string]string{"systemctl": activeState + "\n"}}
		if state := (linuxPlatform{}).StateOfService(machine, blueclaw.CompanyHostTakeOverServiceName); state != want {
			t.Errorf("systemd's %q reads as %s, want %s", activeState, state, want)
		}
	}
}

func TestLaunchdsJobStateBecomesTheThreeStatesACallerActsOn(t *testing.T) {
	for printed, want := range map[string]ServiceState{
		"system/kim.intern.x = {\n\tstate = running\n\tpid = 812\n}":                                   ServiceRunning,
		"system/kim.intern.x = {\n\tstate = not running\n\tlast exit code = 1\n}":                      ServiceFailed,
		"system/kim.intern.x = {\n\tstate = not running\n\tlast exit code = 78: EX_CONFIG\n}":          ServiceFailed,
		"system/kim.intern.x = {\n\tstate = not running\n\tlast exit code = 0\n}":                      ServiceStopped,
		"system/kim.intern.x = {\n\tstate = not running\n\tlast exit code = (never exited)\n}":         ServiceStopped,
		"system/kim.intern.x = {\n\tstate = not running\n\tendpoints = {\n\t\tstate = running\n\t}\n}": ServiceStopped,
	} {
		if state := serviceStateFromLaunchd(printed); state != want {
			t.Errorf("launchd's\n%s\nreads as %s, want %s", printed, state, want)
		}
	}
	machine := launchdMachine{recordedMachine: &recordedMachine{}, loaded: map[string]string{}}
	if state := macPlatformForTest(t).StateOfService(machine, blueclaw.BoxServiceName); state != ServiceStopped {
		t.Fatalf("a label launchd has not loaded reads as %s", state)
	}
}

func TestAMacStartsAServiceByLoadingItAndKicksOneThatWaitsToBeAsked(t *testing.T) {
	platform := macPlatformForTest(t)
	machine := launchdMachine{recordedMachine: &recordedMachine{}, loaded: map[string]string{}}

	if errorValue := platform.StartServices(machine, []string{blueclaw.BoxServiceName, blueclaw.CompanyHostTakeOverServiceName}, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}

	want := []string{
		"launchctl bootstrap system " + filepath.Join(platform.launchDaemonRoot, "kim.intern.internkim-box.plist"),
		"launchctl bootstrap system " + filepath.Join(platform.launchDaemonRoot, "kim.intern.internkim-take-over.plist"),
		"launchctl kickstart system/kim.intern.internkim-take-over",
	}
	if actions := actionsOf(machine.recordedMachine); !slices.Equal(actions, want) {
		t.Fatalf("starting ran %v, want %v", actions, want)
	}
}

func TestAMacStartsAServiceAlreadyLoadedWithoutLoadingItAgain(t *testing.T) {
	machine := launchdMachine{recordedMachine: &recordedMachine{}, loaded: map[string]string{"system/kim.intern.internkim-take-over": "state = not running"}}

	if errorValue := macPlatformForTest(t).StartServices(machine, []string{blueclaw.CompanyHostTakeOverServiceName}, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}

	if actions := actionsOf(machine.recordedMachine); !slices.Equal(actions, []string{"launchctl kickstart system/kim.intern.internkim-take-over"}) {
		t.Fatalf("starting a loaded service ran %v", actions)
	}
}

func TestAMacStopsAServiceByUnloadingItAndLeavesAnUnloadedOneAlone(t *testing.T) {
	machine := launchdMachine{recordedMachine: &recordedMachine{}, loaded: map[string]string{"system/kim.intern.blueclaw": "state = running"}}

	if errorValue := macPlatformForTest(t).StopServices(machine, []string{blueclaw.BlueclawServiceName, blueclaw.ChatdServiceName}, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}

	if actions := actionsOf(machine.recordedMachine); !slices.Equal(actions, []string{"launchctl bootout system/kim.intern.blueclaw"}) {
		t.Fatalf("stopping ran %v; KeepAlive brings back a daemon that is only killed, and bootout refuses one not loaded", actions)
	}
}

func TestAMacEnablesAndDisablesEachLabelInLaunchdsOwnDatabase(t *testing.T) {
	platform := macPlatformForTest(t)
	machine := &recordedMachine{}
	names := []string{blueclaw.BoxServiceName, blueclaw.CompanyHostDatabaseServiceName}
	if errorValue := platform.EnableServices(machine, names); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := platform.DisableServices(machine, names); errorValue != nil {
		t.Fatal(errorValue)
	}
	want := []string{
		"launchctl enable system/kim.intern.internkim-box",
		"launchctl enable system/kim.intern." + blueclaw.CompanyHostDatabaseServiceName,
		"launchctl disable system/kim.intern.internkim-box",
		"launchctl disable system/kim.intern." + blueclaw.CompanyHostDatabaseServiceName,
	}
	if actions := actionsOf(machine); !slices.Equal(actions, want) {
		t.Fatalf("ran %v, want %v", actions, want)
	}
}

func TestAMacWritesBothBoxDaemonsAndStartsOnlyTheBox(t *testing.T) {
	platform := macPlatformForTest(t)
	machine := launchdMachine{recordedMachine: &recordedMachine{}, loaded: map[string]string{"system/kim.intern.internkim-box": "state = running"}}

	if errorValue := platform.SuperviseTheBox(machine, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}

	for _, name := range []string{blueclaw.BoxServiceName, blueclaw.CompanyHostTakeOverServiceName} {
		if _, errorValue := os.Stat(filepath.Join(platform.launchDaemonRoot, "kim.intern."+name+".plist")); errorValue != nil {
			t.Fatalf("no plist was written for %s", name)
		}
	}
	actions := actionsOf(machine.recordedMachine)
	for _, want := range []string{
		"launchctl enable system/kim.intern.internkim-box",
		"launchctl enable system/kim.intern.internkim-take-over",
		"launchctl bootout system/kim.intern.internkim-box",
		"launchctl bootstrap system " + filepath.Join(platform.launchDaemonRoot, "kim.intern.internkim-box.plist"),
	} {
		if !slices.Contains(actions, want) {
			t.Fatalf("supervising the box did not run %q: %v", want, actions)
		}
	}
	for _, action := range actions {
		if strings.Contains(action, "internkim-take-over") && !strings.Contains(action, " enable ") {
			t.Fatalf("supervising the box ran %q, and a take-over starts only when a box is made to serve", action)
		}
	}
}

func TestABoxStandsDownByKeepingTheCompanysServicesFromStartingAndStoppingThem(t *testing.T) {
	machine := &recordedMachine{}
	if errorValue := stopTheCompanyServices(linuxPlatform{}, machine, io.Discard); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(machine.runs) != 2 || machine.runs[0][1] != "disable" || machine.runs[1][1] != "stop" {
		t.Fatalf("standing down ran %v", machine.runs)
	}
	for _, run := range machine.runs {
		for _, left := range []string{blueclaw.BoxServiceName, blueclaw.CompanyHostTakeOverServiceName, blueclaw.CompanyHostDatabaseServiceName, blueclaw.CompanyHostCacheServiceName} {
			if slices.Contains(run, left+".service") {
				t.Fatalf("standing down touches %s: %v", left, run)
			}
		}
		if !slices.Contains(run, blueclaw.BlueclawServiceName+".service") {
			t.Fatalf("standing down leaves the agent running: %v", run)
		}
	}
}

func TestAMacStandbyRefusesToTakeItsCompanyOverByName(t *testing.T) {
	_, errorValue := backupPlatformOf(macPlatformForTest(t))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "macOS") {
		t.Fatalf("a Mac would take its company over: %v", errorValue)
	}
	if _, errorValue := backupPlatformOf(linuxPlatform{}); errorValue != nil {
		t.Fatalf("a Linux box would not take its company over: %v", errorValue)
	}
}
