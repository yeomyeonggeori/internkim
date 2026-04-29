package setup

import (
	"strings"
	"testing"
)

func TestJetsonPreflightAcceptsJetPack6(t *testing.T) {
	output := "__INTERNKIM_PREFLIGHT_BEGIN__\nok\n"
	if errorValue := parseJetsonPreflightOutput(output); errorValue != nil {
		t.Fatalf("expected JetPack 6 preflight output to pass: %v", errorValue)
	}
}

func TestJetsonPreflightRejectsDebianTrixie(t *testing.T) {
	output := "__INTERNKIM_PREFLIGHT_BEGIN__\nfail:Debian Trixie is not supported for Jetson local AI; flash JetPack 6.x / Jetson Linux 36.x\n"
	errorValue := parseJetsonPreflightOutput(output)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "Debian Trixie is not supported") {
		t.Fatalf("expected Debian Trixie rejection, got %v", errorValue)
	}
}

func TestJetsonPreflightReportsMissingSudoClearly(t *testing.T) {
	errorValue := parseJetsonPreflightOutput("sudo: a password is required")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "pass --password") {
		t.Fatalf("expected sudo password guidance, got %v", errorValue)
	}
}
