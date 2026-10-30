package modelladder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestACompanyWithoutItsOwnKeyPaysThroughTheGateway(t *testing.T) {
	directory := t.TempDir()
	keyPath := filepath.Join(directory, "openrouter-key")
	if !PaysThroughGateway(keyPath, "https://plane.example") {
		t.Fatal("a company that brought no key must reach its models through the gateway")
	}
	if errorValue := os.WriteFile(keyPath, []byte("\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !PaysThroughGateway(keyPath, "https://plane.example") {
		t.Fatal("an empty key file is no key, so the company still pays through the gateway")
	}
	if errorValue := os.WriteFile(keyPath, []byte("sk-or-example\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if PaysThroughGateway(keyPath, "https://plane.example") {
		t.Fatal("a company that brought its own key must reach OpenRouter with it")
	}
}

func TestAComputerWithNoCentralPlaneHasNoGatewayToPayThrough(t *testing.T) {
	if PaysThroughGateway(filepath.Join(t.TempDir(), "openrouter-key"), "") {
		t.Fatal("a computer that names no central plane has no gateway to reach")
	}
}

func TestTheGatewayEndpointIsTheModelGatewayFunctionsOpenAIRoot(t *testing.T) {
	endpoint := GatewayEndpoint("https://plane.example/")
	if endpoint != "https://plane.example/functions/v1/model-gateway/api/v1" {
		t.Fatalf("unexpected gateway endpoint %q", endpoint)
	}
	if DecisionsURL(endpoint) != "https://plane.example/functions/v1/model-gateway/api/alpha/decisions" {
		t.Fatalf("the decisions route must stay behind the gateway, got %q", DecisionsURL(endpoint))
	}
}
