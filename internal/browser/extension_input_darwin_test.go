//go:build darwin

package browser

import (
	"fmt"
	"testing"
)

func TestDarwinVirtualKeyCodesCoverDocumentedKeyNames(t *testing.T) {
	requiredKeyNames := []string{"Enter", "Tab", "Escape", "Backspace", "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"}
	for _, keyName := range requiredKeyNames {
		if _, found := darwinVirtualKeyCodes[keyName]; !found {
			t.Errorf("darwinVirtualKeyCodes is missing required key name %q", keyName)
		}
	}
}

func TestDarwinVirtualKeyCodesHaveNoUnexpectedDuplicateCodes(t *testing.T) {
	knownAliasPairs := map[string]string{"Return": "Enter"}
	seenKeyCodes := map[string]string{}
	for keyName, keyCode := range darwinVirtualKeyCodes {
		codeLabel := fmt.Sprintf("%v", keyCode)
		existingKeyName, alreadySeen := seenKeyCodes[codeLabel]
		if alreadySeen && knownAliasPairs[keyName] != existingKeyName && knownAliasPairs[existingKeyName] != keyName {
			t.Errorf("virtual keycode %s is mapped by both %q and %q", codeLabel, existingKeyName, keyName)
		}
		seenKeyCodes[codeLabel] = keyName
	}
}

func TestMouseEventTypesForButtonRejectsUnknownButton(t *testing.T) {
	if _, _, _, errorValue := mouseEventTypesForButton(MouseButton("triple-click")); errorValue == nil {
		t.Fatal("expected an error for an unsupported mouse button name")
	}
}

func TestMouseEventTypesForButtonDefaultsEmptyToLeft(t *testing.T) {
	downType, _, _, errorValue := mouseEventTypesForButton(MouseButton(""))
	if errorValue != nil {
		t.Fatalf("unexpected error: %v", errorValue)
	}
	leftDownType, _, _, _ := mouseEventTypesForButton(MouseButtonLeft)
	if downType != leftDownType {
		t.Fatal("empty MouseButton should behave like MouseButtonLeft")
	}
}
