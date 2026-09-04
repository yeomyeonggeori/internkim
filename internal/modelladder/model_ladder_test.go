package modelladder

import (
	"strings"
	"testing"
)

func TestEveryTierReachesTheSameLadderOnThePlane(t *testing.T) {
	document := LanguageModelDocument("", "/run/example-key")
	if len(document.Tiers) != len(Tiers) {
		t.Fatalf("expected one rung list per tier, got %d", len(document.Tiers))
	}
	for _, tier := range Tiers {
		rungs := document.Tiers[tier]
		if len(rungs) != len(ModelNames()) {
			t.Fatalf("%s: expected the primary model and every degraded one, got %+v", tier, rungs)
		}
		for rungIndex, rung := range rungs {
			if rung.Model != ModelNames()[rungIndex] {
				t.Fatalf("%s: rung %d must be %q, got %q", tier, rungIndex, ModelNames()[rungIndex], rung.Model)
			}
			if rung.Endpoint != Endpoint || rung.APIKeyPath != "/run/example-key" {
				t.Fatalf("%s: rung %d lost its endpoint or key path: %+v", tier, rungIndex, rung)
			}
		}
	}
}

func TestThePlaneAndTheDeviceReachTheSameModel(t *testing.T) {
	planeDocument := LanguageModelDocument("", "/run/example-key")
	deviceModelNames := TierModelNames()
	for _, tier := range Tiers {
		deviceModelName, isNamed := deviceModelNames[tier]
		if !isNamed {
			t.Fatalf("the device names no model for the %s tier", tier)
		}
		if planeDocument.Tiers[tier][0].Model != deviceModelName {
			t.Fatalf("the %s tier runs %q on the plane and %q on a device", tier, planeDocument.Tiers[tier][0].Model, deviceModelName)
		}
	}
}

func TestAnEndpointWithNoKeyPathNamesNone(t *testing.T) {
	document := LanguageModelDocument("", "")
	for _, tier := range Tiers {
		for _, rung := range document.Tiers[tier] {
			if rung.APIKeyPath != "" {
				t.Fatalf("%s named a key path nobody gave it: %q", tier, rung.APIKeyPath)
			}
		}
	}
}

func TestEveryModelNameIsAProviderQualifiedIdentifier(t *testing.T) {
	for _, modelName := range append(ModelNames(), EmbeddingModel, ImageModel) {
		if !strings.Contains(modelName, "/") {
			t.Fatalf("%q is not a model identifier an endpoint would recognize", modelName)
		}
	}
}
