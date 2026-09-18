package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordedCommand struct {
	Name      string
	Arguments []string
}

func recordingRunner(commands *[]recordedCommand) commandRunner {
	return func(name string, arguments ...string) error {
		*commands = append(*commands, recordedCommand{Name: name, Arguments: arguments})
		return nil
	}
}

func TestLaunchdInstallWritesThePlistAndBootstrapsIt(t *testing.T) {
	home := t.TempDir()
	var commands []recordedCommand
	service := launchdService{HomeDirectory: home, UserID: "501", Run: recordingRunner(&commands)}

	if errorValue := service.Install("/usr/local/bin/internkim-companion", []string{"--cua-driver", "/opt/cua driver"}); errorValue != nil {
		t.Fatal(errorValue)
	}

	plist := readTestFile(t, filepath.Join(home, "Library", "LaunchAgents", "kim.intern.companion.plist"))
	for _, expected := range []string{
		"<string>kim.intern.companion</string>",
		"<string>/usr/local/bin/internkim-companion</string>\n\t\t<string>run</string>\n\t\t<string>--cua-driver</string>\n\t\t<string>/opt/cua driver</string>",
		"<string>" + filepath.Join(home, ".local", "bin") + ":/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>",
		"<key>KeepAlive</key>\n\t<true/>",
		"<string>" + filepath.Join(home, "Library", "Logs", "internkim-companion.log") + "</string>",
	} {
		if !strings.Contains(plist, expected) {
			t.Fatalf("plist lacks %q:\n%s", expected, plist)
		}
	}
	expectCommands(t, commands, []recordedCommand{
		{Name: "launchctl", Arguments: []string{"bootout", "gui/501/kim.intern.companion"}},
		{Name: "launchctl", Arguments: []string{"bootstrap", "gui/501", filepath.Join(home, "Library", "LaunchAgents", "kim.intern.companion.plist")}},
	})
}

func TestLaunchdUninstallBootsOutAndRemovesThePlist(t *testing.T) {
	home := t.TempDir()
	var commands []recordedCommand
	service := launchdService{HomeDirectory: home, UserID: "501", Run: recordingRunner(&commands)}
	if errorValue := service.Install("/usr/local/bin/internkim-companion", nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	commands = nil

	if errorValue := service.Uninstall(); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, errorValue := os.Stat(service.plistPath()); !os.IsNotExist(errorValue) {
		t.Fatalf("plist still present after uninstall: %v", errorValue)
	}
	expectCommands(t, commands, []recordedCommand{
		{Name: "launchctl", Arguments: []string{"bootout", "gui/501/kim.intern.companion"}},
	})
	if errorValue := service.Uninstall(); errorValue != nil {
		t.Fatalf("a second uninstall must be a no-op: %v", errorValue)
	}
}

func TestLaunchdRestartKickstartsTheAgent(t *testing.T) {
	var commands []recordedCommand
	service := launchdService{HomeDirectory: t.TempDir(), UserID: "501", Run: recordingRunner(&commands)}
	if errorValue := service.Restart(); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectCommands(t, commands, []recordedCommand{
		{Name: "launchctl", Arguments: []string{"kickstart", "-k", "gui/501/kim.intern.companion"}},
	})
}

func TestLaunchdPlistEscapesArguments(t *testing.T) {
	plist := launchdPlist("/bin/companion", []string{"--name", "a<b>&c"}, "/log", "/bin")
	if !strings.Contains(plist, "<string>a&lt;b&gt;&amp;c</string>") {
		t.Fatalf("argument not escaped:\n%s", plist)
	}
}

func TestSystemdInstallWritesTheUnitAndEnablesIt(t *testing.T) {
	home := t.TempDir()
	var commands []recordedCommand
	service := systemdUserService{HomeDirectory: home, Run: recordingRunner(&commands)}

	if errorValue := service.Install("/usr/local/bin/internkim-companion", []string{"--cua-driver", "/opt/cua driver"}); errorValue != nil {
		t.Fatal(errorValue)
	}

	unit := readTestFile(t, filepath.Join(home, ".config", "systemd", "user", "internkim-companion.service"))
	for _, expected := range []string{
		`ExecStart=/usr/local/bin/internkim-companion run --cua-driver "/opt/cua driver"`,
		"Environment=PATH=" + filepath.Join(home, ".local", "bin") + ":/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin",
		"Restart=always",
		"WantedBy=default.target",
	} {
		if !strings.Contains(unit, expected) {
			t.Fatalf("unit lacks %q:\n%s", expected, unit)
		}
	}
	expectCommands(t, commands, []recordedCommand{
		{Name: "systemctl", Arguments: []string{"--user", "daemon-reload"}},
		{Name: "systemctl", Arguments: []string{"--user", "enable", "--now", "internkim-companion.service"}},
	})
}

func TestSystemdUninstallDisablesAndRemovesTheUnit(t *testing.T) {
	home := t.TempDir()
	var commands []recordedCommand
	service := systemdUserService{HomeDirectory: home, Run: recordingRunner(&commands)}
	if errorValue := service.Install("/usr/local/bin/internkim-companion", nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	commands = nil

	if errorValue := service.Uninstall(); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, errorValue := os.Stat(service.unitPath()); !os.IsNotExist(errorValue) {
		t.Fatalf("unit still present after uninstall: %v", errorValue)
	}
	expectCommands(t, commands, []recordedCommand{
		{Name: "systemctl", Arguments: []string{"--user", "disable", "--now", "internkim-companion.service"}},
		{Name: "systemctl", Arguments: []string{"--user", "daemon-reload"}},
	})
}

func TestSystemdRestartRestartsTheUnit(t *testing.T) {
	var commands []recordedCommand
	service := systemdUserService{HomeDirectory: t.TempDir(), Run: recordingRunner(&commands)}
	if errorValue := service.Restart(); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectCommands(t, commands, []recordedCommand{
		{Name: "systemctl", Arguments: []string{"--user", "restart", "internkim-companion.service"}},
	})
}

func TestQuoteForSystemdLeavesPlainValuesAlone(t *testing.T) {
	if quoted := quoteForSystemd("/usr/local/bin/x"); quoted != "/usr/local/bin/x" {
		t.Fatalf("quoted plain path: %q", quoted)
	}
	if quoted := quoteForSystemd(`say "hi"`); quoted != `"say \"hi\""` {
		t.Fatalf("quoted = %q", quoted)
	}
}

func TestServiceStatusLabel(t *testing.T) {
	if serviceStatusLabel(true) != "running" || serviceStatusLabel(false) != "not running" {
		t.Fatal("status labels changed")
	}
}

func expectCommands(t *testing.T, actual []recordedCommand, expected []recordedCommand) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("ran %d commands, expected %d: %+v", len(actual), len(expected), actual)
	}
	for index := range expected {
		if actual[index].Name != expected[index].Name || strings.Join(actual[index].Arguments, " ") != strings.Join(expected[index].Arguments, " ") {
			t.Fatalf("command %d = %+v, expected %+v", index, actual[index], expected[index])
		}
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(content)
}
