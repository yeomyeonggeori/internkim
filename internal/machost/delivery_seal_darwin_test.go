//go:build darwin

package machost

import (
	"os"
	"path/filepath"
	"testing"
)

func TestASealedShareRefusesTheWriteAGuestWouldMake(t *testing.T) {
	layout, sources := deliverySourcesForTest(t)
	if errorValue := WriteDeliveryDirectory(layout, sources); errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { _ = UnsealDeliveryDirectory(layout) })

	if errorValue := SealDeliveryDirectory(layout); errorValue != nil {
		t.Fatalf("expected the share to seal: %v", errorValue)
	}

	if errorValue := os.WriteFile(layout.RuntimeConfigurationPath(), []byte("rewritten"), 0o644); errorValue == nil {
		t.Fatal("vfkit serves the share as this user, so a write that succeeds here is a write the guest can make")
	}
	if errorValue := os.WriteFile(filepath.Join(layout.DeliverySkillsPath(), "planted.md"), []byte("planted"), 0o644); errorValue == nil {
		t.Fatal("a guest that can add a skill to the share can run code the host never delivered")
	}
}

func TestASealedShareCanStillBeRefreshed(t *testing.T) {
	layout, sources := deliverySourcesForTest(t)
	if errorValue := WriteDeliveryDirectory(layout, sources); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := SealDeliveryDirectory(layout); errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { _ = UnsealDeliveryDirectory(layout) })

	sources.RuntimeConfigurationJSON = `{"guest":{"refreshed":true}}`
	if errorValue := WriteDeliveryDirectory(layout, sources); errorValue != nil {
		t.Fatalf("a seal that cannot be lifted turns every payload update into a reinstall: %v", errorValue)
	}

	document, errorValue := os.ReadFile(layout.RuntimeConfigurationPath())
	if errorValue != nil || string(document) != sources.RuntimeConfigurationJSON {
		t.Fatalf("expected the refreshed document, got %q %v", string(document), errorValue)
	}
}
