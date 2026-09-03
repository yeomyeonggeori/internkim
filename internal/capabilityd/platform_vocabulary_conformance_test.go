package capabilityd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

var platformRoutes = []string{
	"identity.resolve",
	"reply.send",
	"reaction.add",
	"reaction.remove",
	"history.fetch",
	"progress.start",
	"progress.stop",
}

func TestEveryPlatformCapabilitydServesIsDeclared(t *testing.T) {
	service := Service{Configuration: DefaultConfiguration()}

	served := service.platformConnectors()
	if len(served) == 0 {
		t.Fatal("expected capabilityd to serve at least one platform")
	}
	for platform := range served {
		if !capabilityprotocol.IsMessengerPlatform(platform) {
			t.Fatalf("capabilityd serves %q, which the protocol does not declare a messenger", platform)
		}
	}
}

func TestPlatformRoutesRefuseAnUndeclaredPlatform(t *testing.T) {
	service := Service{Configuration: DefaultConfiguration()}
	multiplexer := service.router()

	for _, route := range platformRoutes {
		request := httptest.NewRequest(http.MethodPost, "/v1/platform/a-messenger-nobody-adapts/"+route, strings.NewReader("{}"))
		recorder := httptest.NewRecorder()
		multiplexer.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected %s to refuse an undeclared platform, got %d", route, recorder.Code)
		}
	}
}
