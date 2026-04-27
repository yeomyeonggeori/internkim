package main

import (
	"testing"

	"github.com/anthropic-lab/internkim/internal/capabilities"
)

func TestDefaultCapabilitiesAdvertiseLLMOnlyInDevelopmentMockMode(t *testing.T) {
	withoutMockLLM := defaultCapabilities(true, false)
	withMockLLM := defaultCapabilities(true, true)

	if hasCapability(withoutMockLLM, "llm.structured") {
		t.Fatal("expected LLM capability to be hidden without development mock mode")
	}
	if !hasCapability(withMockLLM, "llm.structured") {
		t.Fatal("expected LLM capability in development mock mode")
	}
	if !hasCapability(withoutMockLLM, "browser.navigate") {
		t.Fatal("expected browser capability to be advertised")
	}
}

func hasCapability(descriptors []capabilities.Descriptor, name string) bool {
	for _, descriptor := range descriptors {
		if descriptor.Name == name {
			return true
		}
	}
	return false
}
