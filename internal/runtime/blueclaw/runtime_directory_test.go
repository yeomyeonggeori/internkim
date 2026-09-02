package blueclaw

import (
	"strings"
	"testing"
)

func TestNoUnitMayDeleteTheRuntimeDirectoryAnotherDaemonUsesToo(t *testing.T) {
	units := map[string]string{
		"capabilityd": CapabilitydServiceUnit(),
		"admind":      AdmindServiceUnit(),
	}
	for name, unit := range units {
		if !strings.Contains(unit, "RuntimeDirectory=internkim") {
			t.Errorf("%s leaves the shared runtime directory to somebody else to create:\n%s", name, unit)
		}
		if !strings.Contains(unit, "RuntimeDirectoryPreserve=yes") {
			t.Errorf("%s stopping would delete the socket the other daemon serves on:\n%s", name, unit)
		}
	}
}
