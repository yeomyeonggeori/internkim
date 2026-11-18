package modelladder

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type whoPaysCase struct {
	HoldsKey      *bool  `json:"holdsKey"`
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
	return func(bool) (bool, error) { return isInUse, nil }
}

func TestTheCompanyKeyPaysOnlyWhenItIsHeldAndInUse(t *testing.T) {
	for _, rule := range whoPaysCases(t) {
		if rule.HoldsKey == nil {
			continue
		}
		keyPath := keyPathHolding(t, "")
		if *rule.HoldsKey {
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

func TestAComputerWithoutAKeySaysSoAndPaysThroughTheGatewayWhateverTheAnswer(t *testing.T) {
	var reported []bool
	unreachable := func(isKeyHeld bool) (bool, error) {
		reported = append(reported, isKeyHeld)
		return true, errors.New("the plane did not answer")
	}
	paysThroughGateway, errorValue := PaysThroughGateway("https://plane.example", keyPathHolding(t, ""), unreachable)
	if errorValue != nil || !paysThroughGateway {
		t.Fatalf("a computer without a key pays through the gateway, got %v, %v", paysThroughGateway, errorValue)
	}
	if len(reported) != 1 || reported[0] {
		t.Fatalf("a computer without a key tells the plane it holds none, reported %v", reported)
	}
}

func TestAComputerThatCannotLearnWhetherToUseItsKeyRefusesToGuess(t *testing.T) {
	unreachable := func(bool) (bool, error) { return false, errors.New("the plane did not answer") }
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

func TestTheGatewayAnswersOpenRoutersPathsUnderItsFunction(t *testing.T) {
	if origin := GatewayOrigin("https://plane.example/"); origin != "https://plane.example/functions/v1/model-gateway" {
		t.Fatalf("unexpected gateway origin %q", origin)
	}
	if origin := OpenRouterOrigin(); origin != "https://openrouter.ai" {
		t.Fatalf("unexpected OpenRouter origin %q", origin)
	}
}

func TestTheModelRouteAnswersTheSamePathsOpenRouterDoes(t *testing.T) {
	endpoint := ModelRouteEndpoint("http://127.0.0.1:18093/")
	if endpoint != "http://127.0.0.1:18093/api/v1" {
		t.Fatalf("unexpected model route endpoint %q", endpoint)
	}
	if DecisionsURL(endpoint) != "http://127.0.0.1:18093/api/alpha/decisions" {
		t.Fatalf("the decisions route must stay behind the model route, got %q", DecisionsURL(endpoint))
	}
}

func TestTheGatewayIsToldWhetherThisComputerHoldsAKeyWithItsCredential(t *testing.T) {
	for _, isKeyHeld := range []bool{true, false} {
		var method, askedPath, authorization string
		var told map[string]bool
		plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			method, askedPath, authorization = request.Method, request.URL.Path, request.Header.Get("Authorization")
			json.NewDecoder(request.Body).Decode(&told)
			writer.Write([]byte(`{"usesOwnModelKey":true}`))
		}))
		credentialPath := keyPathHolding(t, "host-session\n")

		_, errorValue := GatewaySaysWhetherTheCompanyKeyIsInUse(plane.URL+"/", credentialPath)(isKeyHeld)
		plane.Close()

		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if method != http.MethodPost || askedPath != ModelGatewayPayerPath || authorization != "Bearer host-session" || told["holdsModelKey"] != isKeyHeld {
			t.Fatalf("holding a key %v: %s %s with %q told %v", isKeyHeld, method, askedPath, authorization, told)
		}
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
		_, errorValue := reportedTheKey(plane.URL+ModelGatewayPayerPath, keyPathHolding(t, "host-session\n"), true)
		plane.Close()
		if errorValue == nil {
			t.Fatalf("%d %s must not be read as an answer", answer.status, answer.body)
		}
	}
}

func TestTheLadderReachesTheModelRouteWithTheServiceTokenAndNamesTheRequester(t *testing.T) {
	document, errorValue := CompanyLanguageModelDocument("https://plane.example", "http://127.0.0.1:18093", "/run/service-token", "", keyPathHolding(t, "sk-or-example\n"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for tier, rungs := range document.Tiers {
		if len(rungs) != len(ModelNames()) {
			t.Fatalf("tier %s has %d rungs, want %d", tier, len(rungs), len(ModelNames()))
		}
		for _, rung := range rungs {
			if rung.Endpoint != "http://127.0.0.1:18093/api/v1" || rung.APIKeyPath != "/run/service-token" || rung.RequesterEmailHeader != RequesterEmailHeader {
				t.Fatalf("tier %s rung %+v", tier, rung)
			}
		}
	}
	for _, rung := range []Rung{document.Decision, document.VisualReview} {
		if rung.Endpoint != "http://127.0.0.1:18093/api/alpha/decisions" || rung.APIKeyPath != "/run/service-token" || rung.RequesterEmailHeader != RequesterEmailHeader {
			t.Fatalf("decisions and visual review go through the same route, got %+v", rung)
		}
	}
}

func TestALadderOnACentralPlaneWithoutAModelRouteIsRefused(t *testing.T) {
	if _, errorValue := CompanyLanguageModelDocument("https://plane.example", "", "/run/service-token", "", ""); errorValue == nil {
		t.Fatal("a ladder on a central plane has nowhere to reach without the model route")
	}
}

func TestAComputerWithNoCentralPlaneCallsOpenRouterWithTheCompanyKey(t *testing.T) {
	document, errorValue := CompanyLanguageModelDocument("", "http://127.0.0.1:18093", "/run/service-token", "", "/run/example-key")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, rung := range document.Tiers["medium"] {
		if rung.Endpoint != Endpoint || rung.APIKeyPath != "/run/example-key" || rung.RequesterEmailHeader != "" {
			t.Fatalf("a computer with no central plane reached %+v", rung)
		}
	}
}

func TestACompanyRunningItsOwnModelsNeverReachesTheGateway(t *testing.T) {
	document, errorValue := CompanyLanguageModelDocument("https://plane.example", "http://127.0.0.1:18093", "/run/service-token", "https://models.example.com/v1", "/run/example-key")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, rung := range document.Tiers["medium"] {
		if rung.Endpoint != "https://models.example.com/v1" {
			t.Fatalf("a company that runs its own models reached %s", rung.Endpoint)
		}
	}
}

func TestAComputerThePlaneRefusesHearsWhy(t *testing.T) {
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write([]byte(`{"message":"this box belongs to no company"}`))
	}))
	defer plane.Close()
	_, errorValue := GatewaySaysWhetherTheCompanyKeyIsInUse(plane.URL, keyPathHolding(t, "host-session\n"))(true)

	if errorValue == nil || !strings.Contains(errorValue.Error(), "this box belongs to no company") {
		t.Fatalf("the refusal must say what the plane said, got %v", errorValue)
	}
}
