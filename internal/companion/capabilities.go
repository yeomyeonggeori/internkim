package companion

import (
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func DefaultCapabilities(localOnly bool, devMockLLM bool) []capabilities.Descriptor {
	descriptors := capabilities.CompanionToolDescriptors()
	for index, descriptor := range descriptors {
		if strings.HasPrefix(descriptor.Name, "browser_") {
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
		if strings.HasPrefix(descriptor.Name, "browser_") {
			continue
		}
		filteredDescriptors = append(filteredDescriptors, descriptor)
	}
	return filteredDescriptors
}
