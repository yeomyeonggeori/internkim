package blueclaw

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func documentedRelayUnit(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate this test file")
	}
	repositoryRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	document, errorValue := os.ReadFile(filepath.Join(repositoryRoot, "host", "relay", "internkim-relay.service"))
	if errorValue != nil {
		t.Fatalf("read the documented unit: %v", errorValue)
	}
	return string(document)
}

func settingsOf(unit string) map[string]string {
	settings := map[string]string{}
	for _, line := range strings.Split(unit, "\n") {
		name, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found {
			continue
		}
		settings[name] = value
	}
	return settings
}

func TestTheShippedRelayUnitMatchesTheDocumentedOne(t *testing.T) {
	documented := settingsOf(documentedRelayUnit(t))
	shipped := settingsOf(RelayServiceUnit())

	for name, value := range documented {
		if shipped[name] != value {
			t.Errorf("%s is %q in host/relay/internkim-relay.service and %q in the unit this deploys", name, value, shipped[name])
		}
	}
}

func TestRelayStateRemainsWritableWithAProtectedFilesystem(t *testing.T) {
	settings := settingsOf(RelayServiceUnit())
	if settings["ProtectSystem"] != "strict" {
		t.Fatal("the relay lost its filesystem protection")
	}
	if settings["StateDirectory"] != "internkim/relay" || settings["StateDirectoryMode"] != "0750" {
		t.Fatal("systemd must create and expose the relay state directory to its service user")
	}
	if sectionOfEachSetting(RelayServiceUnit())["StateDirectory"] != "Service" {
		t.Fatal("systemd only provisions state directories from the Service section")
	}
}

func sectionOfEachSetting(unit string) map[string]string {
	sections := map[string]string{}
	section := ""
	for _, line := range strings.Split(unit, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.Trim(trimmed, "[]")
			continue
		}
		name, _, found := strings.Cut(trimmed, "=")
		if !found {
			continue
		}
		sections[name] = section
	}
	return sections
}

func TestTheRestartLimitSitsWhereSystemdReadsIt(t *testing.T) {
	units := map[string]string{
		"the unit this deploys":              RelayServiceUnit(),
		"host/relay/internkim-relay.service": documentedRelayUnit(t),
	}
	for what, unit := range units {
		sections := sectionOfEachSetting(unit)
		for _, name := range []string{"StartLimitIntervalSec", "StartLimitBurst"} {
			if sections[name] != "Unit" {
				t.Errorf("%s carries %s in [%s]; systemd reads it only in [Unit] and ignores it silently anywhere else, so the relay would restart without a limit", what, name, sections[name])
			}
		}
	}
}

func TestTheRelayWaitsForItsSettings(t *testing.T) {
	shipped := settingsOf(RelayServiceUnit())
	if shipped["ConditionPathExists"] != RelayEnvironmentFilePath {
		t.Fatalf("a device with no relay settings would start it anyway: %q", shipped["ConditionPathExists"])
	}
	if shipped["EnvironmentFile"] != RelayEnvironmentFilePath {
		t.Fatalf("the unit reads settings from %q", shipped["EnvironmentFile"])
	}
	if shipped["ExecStart"] != RelayBinaryPath {
		t.Fatalf("the unit runs %q", shipped["ExecStart"])
	}
}
