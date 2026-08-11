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
