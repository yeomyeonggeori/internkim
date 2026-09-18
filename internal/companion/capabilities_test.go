package companion

import (
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func TestCapabilitiesWithComputerAddsTheToolToAnOldPairing(t *testing.T) {
	oldPairing := CapabilitiesWithComputer(DefaultCapabilities(false, false), false)
	if countNamed(oldPairing, capabilityprotocol.ComputerTaskToolName) != 0 {
		t.Fatal("computer_task stayed advertised without Cua Driver")
	}
	withDriver := CapabilitiesWithComputer(oldPairing, true)
	if countNamed(withDriver, capabilityprotocol.ComputerTaskToolName) != 1 {
		t.Fatal("computer_task was not advertised with Cua Driver")
	}
	if countNamed(CapabilitiesWithComputer(withDriver, true), capabilityprotocol.ComputerTaskToolName) != 1 {
		t.Fatal("computer_task was advertised twice")
	}
}

func countNamed(descriptors []capabilities.Descriptor, name string) int {
	count := 0
	for _, descriptor := range descriptors {
		if descriptor.Name == name {
			count++
		}
	}
	return count
}
