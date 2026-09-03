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
	guestConfiguration, isPresent := runtimeConfiguration["guest"].(map[string]any)
	if !isPresent {
		t.Fatal("expected the guest configuration block")
	}

	if deliveryDirectoryPath, _ := guestConfiguration["deliveryDirectoryPath"].(string); deliveryDirectoryPath == "" {
		t.Fatal("Cloud Hypervisor serves the delivery directory over virtio-fs, so the share must be named")
	}
	if guestConfiguration["virtiofsdPath"] != BlueclawVirtiofsdPath {
		t.Fatalf("expected the daemon path the supervisor starts, got %v", guestConfiguration["virtiofsdPath"])
	}
}
