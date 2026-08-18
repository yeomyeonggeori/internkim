package cli

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/releaseset"
)

func carriedComponents(t *testing.T, rebuilt map[string]releaseset.Component, published map[string]releaseset.Component, revisionOf func(string) string) (map[string]releaseset.Component, error) {
	t.Helper()
	return rebuilt, carryComponentsForward(rebuilt, published, revisionOf)
}

func TestAComponentThatStillMatchesTheTreeIsCarriedForward(t *testing.T) {
	rebuilt := map[string]releaseset.Component{"relay": {Name: "relay", Revision: "tree-1"}}
	published := map[string]releaseset.Component{
		"relay": {Name: "relay", Revision: "old"},
		"chatd": {Name: "chatd", Revision: "blueclaw-1"},
	}

	carried, errorValue := carriedComponents(t, rebuilt, published, func(name string) string {
		return map[string]string{"relay": "tree-1", "chatd": "blueclaw-1"}[name]
	})
	if errorValue != nil {
		t.Fatalf("nothing had moved, yet: %v", errorValue)
	}
	if carried["chatd"].Revision != "blueclaw-1" {
		t.Fatalf("the release must still name chatd, got %+v", carried)
	}
}

func TestAComponentTheTreeHasMovedPastStopsTheRelease(t *testing.T) {
	rebuilt := map[string]releaseset.Component{"relay": {Name: "relay", Revision: "tree-2"}}
	published := map[string]releaseset.Component{
		"relay": {Name: "relay", Revision: "tree-1"},
		"chatd": {Name: "chatd", Revision: "blueclaw-1"},
	}

	_, errorValue := carriedComponents(t, rebuilt, published, func(name string) string {
		return map[string]string{"relay": "tree-2", "chatd": "blueclaw-2"}[name]
	})
	if errorValue == nil {
		t.Fatal("a relay shipped beside a chatd built from an older blueclaw was accepted")
	}
	if !strings.Contains(errorValue.Error(), "chatd") {
		t.Fatalf("the refusal must name the component to rebuild: %v", errorValue)
	}
}

func TestTheFirstReleaseOfAComponentIsNotMissed(t *testing.T) {
	rebuilt := map[string]releaseset.Component{"relay": {Name: "relay", Revision: "tree-1"}}
	published := map[string]releaseset.Component{"relay": {Name: "relay", Revision: "tree-1"}}

	carried, errorValue := carriedComponents(t, rebuilt, published, func(string) string { return "tree-1" })
	if errorValue != nil {
		t.Fatalf("a component the device has never had must not stop a release: %v", errorValue)
	}
	if _, named := carried["chatd"]; named {
		t.Fatal("a component that was never published cannot be carried forward")
	}
}

func TestAComponentThisTreeCannotBuildIsCarriedRatherThanBlocking(t *testing.T) {
	rebuilt := map[string]releaseset.Component{"chatd": {Name: "chatd", Revision: "blueclaw-2"}}
	published := map[string]releaseset.Component{
		"chatd":           {Name: "chatd", Revision: "blueclaw-1"},
		"blueclawPayload": {Name: "blueclawPayload", Revision: "payload-7"},
	}

	// The payload artifact is not in this tree, so it has no revision to offer.
	carried, errorValue := carriedComponents(t, rebuilt, published, func(name string) string {
		return map[string]string{"chatd": "blueclaw-2", "blueclawPayload": ""}[name]
	})
	if errorValue != nil {
		t.Fatalf("a component this tree cannot speak for stopped an unrelated deploy: %v", errorValue)
	}
	if carried["blueclawPayload"].Revision != "payload-7" {
		t.Fatalf("the device must keep the payload it has, got %+v", carried["blueclawPayload"])
	}
}
