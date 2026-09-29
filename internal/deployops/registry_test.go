package deployops

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTheVaultNamesTheOneDeviceTarget(t *testing.T) {
	t.Setenv(DeviceURLVariable, "https://fleetexample.example.test/")
	t.Setenv(SSHHostnameVariable, "0.ssh.fleetexample.example.test")
	t.Setenv(FleetIDVariable, "FleetExample")
	t.Setenv(FleetSecretVariable, "example-fleet-secret")

	registry := LoadRegistry()

	if len(registry.Targets) != 1 {
		t.Fatalf("expected the one device the vault names, got %d", len(registry.Targets))
	}
	target := registry.Targets[0]
	if target.ID != "fleetexample" || target.FleetID != "fleetexample" || target.AdminURL != "https://fleetexample.example.test" {
		t.Fatalf("unexpected target: %+v", target)
	}
	if target.SSHHostname != "0.ssh.fleetexample.example.test" || target.FleetSecret != "example-fleet-secret" {
		t.Fatalf("the target lost what the vault named: %+v", target)
	}
}

func TestAVaultNamingNoDeviceHasNoTarget(t *testing.T) {
	t.Setenv(DeviceURLVariable, "")
	t.Setenv(FleetSecretVariable, "example-fleet-secret")

	if registry := LoadRegistry(); len(registry.Targets) != 0 {
		t.Fatalf("a device without an address became a target: %+v", registry.Targets)
	}
}

func TestTheConsoleNeverServesTheFleetSecret(t *testing.T) {
	document, errorValue := json.Marshal(Target{ID: "fleetexample", FleetSecret: "example-fleet-secret"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(document), "example-fleet-secret") {
		t.Fatalf("the target list carries the fleet secret: %s", document)
	}
}

func TestATargetWithoutFleetIdentityCannotSign(t *testing.T) {
	_, errorValue := targetIdentity(Target{Name: "fleetexample.example.test", FleetID: "fleetexample"})
	if errorValue == nil || !strings.Contains(errorValue.Error(), FleetSecretVariable) {
		t.Fatalf("expected the refusal to name the missing key, got %v", errorValue)
	}
}
