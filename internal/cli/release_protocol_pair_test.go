package cli

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/releaseset"
)

func componentsNamed(names ...string) map[string]releaseset.Component {
	components := map[string]releaseset.Component{}
	for _, name := range names {
		components[name] = releaseset.Component{Revision: "revision-" + name}
	}
	return components
}

// 2026-09-01: capabilityd went out alone, the guest kept the payload it had,
// and blueclaw refused every message for forty minutes while systemctl and the
// admin URL both said it was fine.
func TestReleaseRefusesToReplaceOnlyOneSideOfTheProtocol(t *testing.T) {
	errorValue := refuseToSplitTheProtocol(
		componentsNamed("admind", "capabilityd"),
		componentsNamed("admind", "capabilityd", "blueclawPayload"),
	)
	if errorValue == nil {
		t.Fatal("a release replacing capabilityd alone was allowed")
	}
	if !strings.Contains(errorValue.Error(), "blueclawPayload") {
		t.Fatalf("the refusal does not name what to add: %v", errorValue)
	}
	if !strings.Contains(errorValue.Error(), "prepare-blueclaw-payload") {
		t.Fatalf("the refusal does not say how to build it: %v", errorValue)
	}
}

func TestReleaseRefusesToReplaceOnlyTheGuest(t *testing.T) {
	errorValue := refuseToSplitTheProtocol(
		componentsNamed("blueclawPayload"),
		componentsNamed("capabilityd", "blueclawPayload"),
	)
	if errorValue == nil {
		t.Fatal("a release replacing the guest alone was allowed")
	}
	if !strings.Contains(errorValue.Error(), "capabilityd") {
		t.Fatalf("the refusal does not name what to add: %v", errorValue)
	}
}

func TestReleaseCarryingBothSidesForwardIsFine(t *testing.T) {
	if errorValue := refuseToSplitTheProtocol(
		componentsNamed("web"),
		componentsNamed("web", "capabilityd", "blueclawPayload"),
	); errorValue != nil {
		t.Fatalf("a release touching neither side was refused: %v", errorValue)
	}
}

func TestReleaseReplacingBothSidesIsFine(t *testing.T) {
	if errorValue := refuseToSplitTheProtocol(
		componentsNamed("capabilityd", "blueclawPayload"),
		componentsNamed("capabilityd", "blueclawPayload"),
	); errorValue != nil {
		t.Fatalf("a release replacing both sides was refused: %v", errorValue)
	}
}

// A device that has never run the guest agent has no payload to keep in step.
func TestReleaseDoesNotAskForAComponentTheDeviceNeverHad(t *testing.T) {
	if errorValue := refuseToSplitTheProtocol(
		componentsNamed("capabilityd"),
		componentsNamed("capabilityd"),
	); errorValue != nil {
		t.Fatalf("a device without the guest payload was refused: %v", errorValue)
	}
}
