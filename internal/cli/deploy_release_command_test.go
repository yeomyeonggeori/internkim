package cli

import "testing"

func TestSelectedReleaseComponentNamesParsesCommaList(t *testing.T) {
	components, errorValue := selectedReleaseComponentNames([]string{"--components", "capabilityd,admind"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !components["capabilityd"] || !components["admind"] {
		t.Fatalf("components = %+v", components)
	}
	if components["skills"] {
		t.Fatalf("unexpected skills component in %+v", components)
	}
}

func TestSelectedReleaseComponentNamesAllowsAllByDefault(t *testing.T) {
	components, errorValue := selectedReleaseComponentNames(nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if components != nil {
		t.Fatalf("components = %+v, want nil for all components", components)
	}
}
