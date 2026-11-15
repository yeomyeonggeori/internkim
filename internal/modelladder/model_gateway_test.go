package modelladder

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type whoPaysCase struct {
	HoldsKey      bool   `json:"holdsKey"`
	UsesOwnKey    bool   `json:"usesOwnKey"`
	CreditsCanPay bool   `json:"creditsCanPay"`
	PaidBy        string `json:"paidBy"`
}

func whoPaysCases(t *testing.T) []whoPaysCase {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join("testdata", "who-pays.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var cases []whoPaysCase
	if errorValue := json.Unmarshal(document, &cases); errorValue != nil {
		t.Fatal(errorValue)
	}
	return cases
}

func keyPathHolding(t *testing.T, contents string) string {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "openrouter-key")
	if contents == "" {
		return keyPath
	}
	if errorValue := os.WriteFile(keyPath, []byte(contents), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return keyPath
}

func answering(isInUse bool) CompanyKeyUse {
	return func() (bool, error) { return isInUse, nil }
}

func TestTheCompanyKeyPaysOnlyWhenItIsHeldAndInUse(t *testing.T) {
	for _, rule := range whoPaysCases(t) {
		keyPath := keyPathHolding(t, "")
		if rule.HoldsKey {
			keyPath = keyPathHolding(t, "sk-or-example\n")
		}
		paysThroughGateway, errorValue := PaysThroughGateway("https://plane.example", keyPath, answering(rule.UsesOwnKey))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if paysThroughGateway != (rule.PaidBy != "company_key") {
			t.Fatalf("%+v: paying through the gateway is %v", rule, paysThroughGateway)
		}
	}
}

func TestAnEmptyKeyFileIsNoKey(t *testing.T) {
	paysThroughGateway, errorValue := PaysThroughGateway("https://plane.example", keyPathHolding(t, "\n"), answering(true))
	if errorValue != nil || !paysThroughGateway {
		t.Fatalf("an empty key file must leave the company paying through the gateway, got %v, %v", paysThroughGateway, errorValue)
	}
}

func TestAComputerWithoutAKeyNeverAsksWhetherToUseOne(t *testing.T) {
	refusing := func() (bool, error) {
		t.Fatal("a computer that holds no key has nothing to ask about")
		return false, nil
	}
	paysThroughGateway, errorValue := PaysThroughGateway("https://plane.example", keyPathHolding(t, ""), refusing)
	if errorValue != nil || !paysThroughGateway {
		t.Fatalf("a computer without a key pays through the gateway, got %v, %v", paysThroughGateway, errorValue)
	}
}

func TestAComputerThatCannotLearnWhetherToUseItsKeyRefusesToGuess(t *testing.T) {
	unreachable := func() (bool, error) { return false, errors.New("the plane did not answer") }
	_, errorValue := PaysThroughGateway("https://plane.example", keyPathHolding(t, "sk-or-example\n"), unreachable)
	if errorValue == nil {
		t.Fatal("a computer holding a key must not guess who pays when the plane cannot say")
	}
}

func TestAComputerWithNoCentralPlaneHasNoGatewayToPayThrough(t *testing.T) {
	paysThroughGateway, errorValue := PaysThroughGateway("", keyPathHolding(t, ""), nil)
	if errorValue != nil || paysThroughGateway {
		t.Fatalf("a computer that names no central plane has no gateway to reach, got %v, %v", paysThroughGateway, errorValue)
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

func TestTheGatewayIsAskedWithThisComputersCredential(t *testing.T) {
	var askedPath, authorization string
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		askedPath, authorization = request.URL.Path, request.Header.Get("Authorization")
		writer.Write([]byte(`{"usesOwnModelKey":true}`))
	}))
	defer plane.Close()
	credentialPath := keyPathHolding(t, "host-session\n")

	isInUse, errorValue := GatewaySaysWhetherTheCompanyKeyIsInUse(plane.URL+"/", credentialPath)()

	if errorValue != nil || !isInUse {
		t.Fatalf("the gateway said the key is in use, got %v, %v", isInUse, errorValue)
	}
	if askedPath != ModelGatewayPayerPath || authorization != "Bearer host-session" {
		t.Fatalf("asked %s with %q", askedPath, authorization)
	}
}

func TestARefusingGatewayIsAnErrorRatherThanAnAnswer(t *testing.T) {
	for _, answer := range []struct {
		status int
		body   string
	}{
		{http.StatusForbidden, `{"message":"refused"}`},
		{http.StatusOK, `{}`},
	} {
		plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(answer.status)
			writer.Write([]byte(answer.body))
		}))
		_, errorValue := GatewaySaysWhetherTheCompanyKeyIsInUse(plane.URL, keyPathHolding(t, "host-session\n"))()
		plane.Close()
		if errorValue == nil {
			t.Fatalf("%d %s must not be read as an answer", answer.status, answer.body)
		}
	}
}

func TestTheLadderReachesOnlyTheOneWhoPays(t *testing.T) {
	keyPath := keyPathHolding(t, "sk-or-example\n")
	gateway := GatewayEndpoint("https://plane.example")
	for _, isInUse := range []bool{true, false} {
		document, errorValue := CompanyLanguageModelDocument("https://plane.example", "/run/session", "", keyPath, answering(isInUse))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		wantEndpoint, wantKey, wantHeader := gateway, "/run/session", RequesterEmailHeader
		if isInUse {
			wantEndpoint, wantKey, wantHeader = Endpoint, keyPath, ""
		}
		for tier, rungs := range document.Tiers {
			if len(rungs) != len(ModelNames()) {
				t.Fatalf("tier %s has %d rungs, want one payer's %d", tier, len(rungs), len(ModelNames()))
			}
			for _, rung := range rungs {
				if rung.Endpoint != wantEndpoint || rung.APIKeyPath != wantKey || rung.RequesterEmailHeader != wantHeader {
					t.Fatalf("key in use %v: tier %s rung %+v", isInUse, tier, rung)
				}
			}
		}
		for _, rung := range []Rung{document.Decision, document.VisualReview} {
			if rung.APIKeyPath != wantKey || rung.RequesterEmailHeader != wantHeader {
				t.Fatalf("key in use %v: decisions and visual review follow the same payer, got %+v", isInUse, rung)
			}
		}
	}
}

func TestACompanyRunningItsOwnModelsNeverReachesTheGateway(t *testing.T) {
	document, errorValue := CompanyLanguageModelDocument("https://plane.example", "/run/session", "https://models.example.com/v1", "/run/example-key", nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, rung := range document.Tiers["medium"] {
		if rung.Endpoint != "https://models.example.com/v1" {
			t.Fatalf("a company that runs its own models reached %s", rung.Endpoint)
		}
	}
}
