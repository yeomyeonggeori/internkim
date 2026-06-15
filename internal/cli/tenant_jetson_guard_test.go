package cli

import (
	"strings"
	"testing"
)

func TestHostRuntimeProvisionGuardRefusesOnJetsonBoard(t *testing.T) {
	if hostRuntimeProvisionGuardError(false) != nil {
		t.Fatal("non-Jetson host must be allowed to install host-runtime tenants")
	}
	errorValue := hostRuntimeProvisionGuardError(true)
	if errorValue == nil {
		t.Fatal("a Jetson production board must refuse host-runtime tenant install")
	}
	if !strings.Contains(errorValue.Error(), "nspawn") || !strings.Contains(errorValue.Error(), "Jetson") {
		t.Fatalf("guard error should explain to use nspawn on the Mac host, got %q", errorValue.Error())
	}
}
