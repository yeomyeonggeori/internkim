package companion

import (
	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func DefaultCapabilities(localOnly bool, devMockLLM bool) []capabilities.Descriptor {
	descriptors := capabilities.CompanionToolDescriptors()
	for index, descriptor := range descriptors {
		if descriptor.Namespace == browserCapabilityNamespace {
			descriptors[index].WorksOffline = localOnly
		}
	}
	if devMockLLM {
		descriptors = append(descriptors, capabilities.CompanionLLMDescriptors()...)
	}
	return descriptors
}

func CapabilitiesWithoutBrowser(descriptors []capabilities.Descriptor) []capabilities.Descriptor {
	filteredDescriptors := []capabilities.Descriptor{}
	for _, descriptor := range descriptors {
		if descriptor.Namespace == browserCapabilityNamespace {
			continue
		}
		filteredDescriptors = append(filteredDescriptors, descriptor)
	}
	return filteredDescriptors
}

// A browser capability is one the browser namespace owns, which is identity, not
// a guess made from the shape of a name.
const browserCapabilityNamespace = "browser"

// CapabilitiesWithComputer makes the advertised list say whether this run can
// control the computer, whatever the pairing recorded: old pairings never
// listed computer_task, and a run without Cua Driver must not offer it.
func CapabilitiesWithComputer(descriptors []capabilities.Descriptor, isComputerControlAvailable bool) []capabilities.Descriptor {
	withoutComputer := []capabilities.Descriptor{}
	for _, descriptor := range descriptors {
		if descriptor.Name == capabilityprotocol.ComputerTaskToolName {
			continue
		}
		withoutComputer = append(withoutComputer, descriptor)
	}
	if !isComputerControlAvailable {
		return withoutComputer
	}
	return append(withoutComputer, computerTaskDescriptor())
}

func computerTaskDescriptor() capabilities.Descriptor {
	for _, descriptor := range capabilities.CompanionToolDescriptors() {
		if descriptor.Name == capabilityprotocol.ComputerTaskToolName {
			return descriptor
		}
	}
	panic("the companion tool descriptors no longer include " + capabilityprotocol.ComputerTaskToolName)
}
