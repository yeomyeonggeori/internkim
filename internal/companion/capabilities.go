package companion

import (
	"gitlab.com/eastriver/internkim/internal/capabilities"
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
