package blueclaw

import (
	"encoding/json"
	"testing"
)

func TestRuntimeConfigurationNamesTheDeliveryShareOnlyWhenTheMonitorServesIt(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocument("")
	if errorValue != nil {
		t.Fatalf("expected a runtime document: %v", errorValue)
	}
	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatalf("expected the runtime document to parse: %v", errorValue)
	}
	guestConfiguration, isPresent := runtimeConfiguration["firecracker"].(map[string]any)
	if !isPresent {
		t.Fatal("expected the guest configuration block")
	}

	deliveryDirectoryPath, _ := guestConfiguration["deliveryDirectoryPath"].(string)
	if BlueclawVirtualMachineMonitor == FirecrackerMonitorName {
		if deliveryDirectoryPath != "" {
			t.Fatal("Firecracker emulates no virtio-fs, so naming a share asks for a device it cannot offer")
		}
	} else if deliveryDirectoryPath != BlueclawDeliveryReadOnlyPath {
		t.Fatalf("expected the guest to be offered the read-only bind, got %q", deliveryDirectoryPath)
	}

	if guestConfiguration["virtiofsdPath"] != BlueclawVirtiofsdPath {
		t.Fatalf("expected the daemon path the supervisor starts, got %v", guestConfiguration["virtiofsdPath"])
	}
}
