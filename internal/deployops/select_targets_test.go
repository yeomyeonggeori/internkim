package deployops

import (
	"strings"
	"testing"
)

func TestSelectTargetsReturnsAllWhenFleetIDsAreEmpty(t *testing.T) {
	registry := TargetRegistry{Targets: []Target{
		{ID: "jetson-1", AdminURL: "https://jetson-1.example"},
		{ID: "poc-1", Kind: "poc-container", SSHHost: "studio.local"},
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
		{ID: "poc-1", Kind: "poc-container", SSHHost: "studio.local"},
	}}

	targets, errorValue := SelectTargets(registry, []string{"poc-1"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(targets) != 1 || targets[0].ID != "poc-1" {
		t.Fatalf("targets = %#v, want poc-1 only", targets)
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

func TestTargetResolvedKindDefaultsToJetson(t *testing.T) {
	if kind := (Target{}).ResolvedKind(); kind != "jetson" {
		t.Fatalf("kind = %q, want jetson", kind)
	}
	if kind := (Target{Kind: "poc-container"}).ResolvedKind(); kind != "poc-container" {
		t.Fatalf("kind = %q, want poc-container", kind)
	}
}
