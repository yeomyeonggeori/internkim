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

func TestSubscriptionCreditsPayBeforeTheCompanysOwnKey(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "openrouter-key")
	if errorValue := os.WriteFile(keyPath, []byte("sk-or-example\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	document := CompanyLanguageModelDocument("https://plane.example", "/run/session", "", keyPath)
	gateway := GatewayEndpoint("https://plane.example")
	for _, tier := range Tiers {
		rungs := document.Tiers[tier]
		modelCount := len(ModelNames())
		if len(rungs) != 2*modelCount {
			t.Fatalf("tier %s has %d rungs, want the gateway's and then the company's own", tier, len(rungs))
		}
		for index, rung := range rungs {
			wantEndpoint, wantKey := gateway, "/run/session"
			if index >= modelCount {
				wantEndpoint, wantKey = Endpoint, keyPath
			}
			if rung.Endpoint != wantEndpoint || rung.APIKeyPath != wantKey {
				t.Fatalf("tier %s rung %d reaches %s with %s, want %s with %s", tier, index, rung.Endpoint, rung.APIKeyPath, wantEndpoint, wantKey)
			}
		}
	}
	if document.Decision.APIKeyPath != keyPath {
		t.Fatalf("a decision has no rung to fall back to, so it is paid with the company's key, got %q", document.Decision.APIKeyPath)
	}
}

func TestACompanyWithoutItsOwnKeyReachesOnlyTheGateway(t *testing.T) {
	document := CompanyLanguageModelDocument("https://plane.example", "/run/session", "", filepath.Join(t.TempDir(), "openrouter-key"))
	for _, rung := range document.Tiers["medium"] {
		if rung.Endpoint != GatewayEndpoint("https://plane.example") {
			t.Fatalf("a company with no key of its own reached %s", rung.Endpoint)
		}
	}
}

func TestACompanyRunningItsOwnModelsNeverReachesTheGateway(t *testing.T) {
	document := CompanyLanguageModelDocument("https://plane.example", "/run/session", "https://models.example.com/v1", "/run/example-key")
	for _, rung := range document.Tiers["medium"] {
		if rung.Endpoint != "https://models.example.com/v1" {
			t.Fatalf("a company that runs its own models reached %s", rung.Endpoint)
		}
	}
}

func TestOnlyTheGatewayIsToldWhoAskedForACall(t *testing.T) {
	directory := t.TempDir()
	companyKeyPath := filepath.Join(directory, "openrouter-key")
	if errorValue := os.WriteFile(companyKeyPath, []byte("sk-or-example\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	gateway := GatewayEndpoint("https://plane.example")

	document := CompanyLanguageModelDocument("https://plane.example", filepath.Join(directory, "host-session"), "", companyKeyPath)

	for tier, rungs := range document.Tiers {
		for _, rung := range rungs {
			isGateway := rung.Endpoint == gateway
			if isGateway && rung.RequesterEmailHeader != RequesterEmailHeader {
				t.Fatalf("tier %s: a gateway rung must name the requester, got %+v", tier, rung)
			}
			if !isGateway && rung.RequesterEmailHeader != "" {
				t.Fatalf("tier %s: a rung that reaches OpenRouter with the company's key must not send who asked, got %+v", tier, rung)
			}
		}
	}
	if document.Decision.RequesterEmailHeader != "" {
		t.Fatalf("a company with its own key decides with it and names no requester, got %+v", document.Decision)
	}

	keyless := CompanyLanguageModelDocument("https://plane.example", filepath.Join(directory, "host-session"), "", filepath.Join(directory, "absent"))
	if keyless.Decision.RequesterEmailHeader != RequesterEmailHeader || keyless.VisualReview.RequesterEmailHeader != RequesterEmailHeader {
		t.Fatalf("decisions through the gateway must name the requester, got %+v and %+v", keyless.Decision, keyless.VisualReview)
	}
}
