package cli

import (
	"testing"

	"gitlab.com/eastriver/internkim/internal/deployops"
)

func TestDeployFleetIDsParsesCommaSeparatedAndRepeatableValues(t *testing.T) {
	fleetIDs := deployFleetIDs([]string{"--fleet", "a,b", "--fleet=c", "--fleet", "d"})

	expectedFleetIDs := []string{"a", "b", "c", "d"}
	if len(fleetIDs) != len(expectedFleetIDs) {
		t.Fatalf("fleetIDs = %#v, want %#v", fleetIDs, expectedFleetIDs)
	}
	for index, expectedFleetID := range expectedFleetIDs {
		if fleetIDs[index] != expectedFleetID {
			t.Fatalf("fleetIDs = %#v, want %#v", fleetIDs, expectedFleetIDs)
		}
	}
}

func TestDeployFleetFlagIsAcceptedByValidation(t *testing.T) {
	if errorValue := validateDeployArguments([]string{"--fleet", "a,b", "--fleet=c"}); errorValue != nil {
		t.Fatalf("validateDeployArguments returned error: %v", errorValue)
	}
}

func TestSelectRegistryDeployTargetsFallsBackWhenRegistryIsEmpty(t *testing.T) {
	_, shouldUseRegistry, errorValue := selectRegistryDeployTargets(deployops.TargetRegistry{}, []string{"--components", "admind"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if shouldUseRegistry {
		t.Fatal("expected empty registry to fall back to single-target deploy")
	}
}
