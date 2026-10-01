package capabilityd

import (
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func TestEveryRegisteredToolHasARoute(t *testing.T) {
	for _, descriptor := range capabilities.DefaultToolDescriptors() {
		if _, hasRoute := capabilityToolRouteFor(descriptor.CanonicalName); !hasRoute {
			t.Errorf("%s is in the catalog but no route serves it, so an agent that calls it is told the tool is not configured", descriptor.CanonicalName)
		}
	}
}

func TestEveryRouteHasADescriptor(t *testing.T) {
	for _, route := range capabilityToolRoutes {
		if _, hasDescriptor := capabilityToolDescriptorFor(route.ToolName); !hasDescriptor {
			t.Errorf("%s has a route but no descriptor, so the catalog never offers it", route.ToolName)
		}
	}
}
