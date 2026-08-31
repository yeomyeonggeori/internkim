package deployops

import (
	"strings"
	"testing"
)

func TestSelectTargetsReturnsAllWhenFleetIDsAreEmpty(t *testing.T) {
	registry := TargetRegistry{Targets: []Target{
		{ID: "jetson-1", AdminURL: "https://jetson-1.example"},
		{ID: "jetson-2", AdminURL: "https://jetson-2.example"},
	}}

	targets, errorValue := SelectTargets(registry, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(targets))
	}
}

func TestSelectTargetsReturnsSubsetByID(t *testing.T) {
	registry := TargetRegistry{Targets: []Target{
		{ID: "jetson-1", AdminURL: "https://jetson-1.example"},
		{ID: "jetson-2", AdminURL: "https://jetson-2.example"},
	}}

	targets, errorValue := SelectTargets(registry, []string{"jetson-2"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(targets) != 1 || targets[0].ID != "jetson-2" {
		t.Fatalf("targets = %#v, want jetson-2 only", targets)
	}
}

func TestSelectTargetsReturnsErrorForUnknownID(t *testing.T) {
	registry := TargetRegistry{Targets: []Target{{ID: "jetson-1", AdminURL: "https://jetson-1.example"}}}

	_, errorValue := SelectTargets(registry, []string{"missing"})
	if errorValue == nil {
		t.Fatal("expected unknown target error")
	}
	if !strings.Contains(errorValue.Error(), "missing") {
		t.Fatalf("error = %q, want missing target id", errorValue.Error())
	}
}

func TestATargetNamingNoKindIsADevice(t *testing.T) {
	if kind := (Target{}).ResolvedKind(); kind != deviceTargetKind {
		t.Fatalf("kind = %q, want %q", kind, deviceTargetKind)
	}
}

// A registry kept on somebody's machine can still name a kind this repository
// retired. It is read back as what it says rather than taken for a device.
func TestATargetNamingARetiredKindKeepsSayingSo(t *testing.T) {
	if kind := (Target{Kind: "poc-container"}).ResolvedKind(); kind != "poc-container" {
		t.Fatalf("kind = %q, want the kind the registry named", kind)
	}
}
