package admind

import (
	"testing"

	"gitlab.com/eastriver/internkim/internal/releaseset"
)

func releaseOf(components map[string]string) *releaseset.Manifest {
	held := map[string]releaseset.Component{}
	for name, digest := range components {
		held[name] = releaseset.Component{Name: name, SHA256: digest}
	}
	return &releaseset.Manifest{ReleaseID: "release-1", Components: held}
}

func TestOnlyWhatDiffersIsInstalled(t *testing.T) {
	wanted := releaseOf(map[string]string{"admind": "new", "blueclawPayload": "same", "chatd": "same"})
	installed := releaseOf(map[string]string{"admind": "old", "blueclawPayload": "same", "chatd": "same"})

	changed := componentsNotAlreadyInstalled(wanted, installed)

	if len(changed.Components) != 1 {
		t.Fatalf("only admind changed, got %+v", changed.Components)
	}
	if _, named := changed.Components["admind"]; !named {
		t.Fatalf("admind must be installed, got %+v", changed.Components)
	}
}

func TestACarriedComponentThatCannotBeInstalledDoesNotBlockTheRest(t *testing.T) {
	// The device holds a payload its own admind refuses to install again. Every
	// release carries that payload forward, so installing it every time would
	// mean no release could ever reach the device, including the one that
	// replaces the admind refusing it.
	wanted := releaseOf(map[string]string{"admind": "new", "blueclawPayload": "the-one-it-refuses"})
	installed := releaseOf(map[string]string{"admind": "old", "blueclawPayload": "the-one-it-refuses"})

	changed := componentsNotAlreadyInstalled(wanted, installed)

	if _, named := changed.Components["blueclawPayload"]; named {
		t.Fatal("a payload the device already has must not be installed again")
	}
}

func TestTheFirstReleaseInstallsEverything(t *testing.T) {
	wanted := releaseOf(map[string]string{"admind": "new", "chatd": "new"})

	changed := componentsNotAlreadyInstalled(wanted, nil)

	if len(changed.Components) != 2 {
		t.Fatalf("a device with no release yet takes all of it, got %+v", changed.Components)
	}
}

func TestTheReleaseItselfIsUnchanged(t *testing.T) {
	wanted := releaseOf(map[string]string{"admind": "new", "chatd": "same"})
	installed := releaseOf(map[string]string{"chatd": "same"})

	componentsNotAlreadyInstalled(wanted, installed)

	if len(wanted.Components) != 2 {
		t.Fatalf("the release recorded afterwards must still name every component, got %+v", wanted.Components)
	}
}
