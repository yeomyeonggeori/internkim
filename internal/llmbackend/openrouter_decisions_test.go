package llmbackend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/modelladder"
)

const decisionsAnswerDocument = `{
  "id": "gen-dec-1789697042-s7VZqLokxV3PsVjUcjjQ",
  "model": "typesafe/jev-1.13-20260917",
  "provider": "TypeSafe",
  "answers": {
    "target": {"type": "choice", "choice": "bot", "probabilities": {"bot": 0.98, "human": 0.02}, "confidence": 0.97},
    "shouldRespond": {"type": "noul", "noul": 0},
    "level": {"type": "score", "score": 1.4, "probabilities": {"low": 0.6, "high": 0.4}, "confidence": 0.5, "legend": {"0": "low"}}
  },
  "usage": {"input_tokens": 2285, "output_tokens": 826, "cost": 9.597e-05}
}`

func decisionsStandIn(t *testing.T, statusCode int, responseBody string) (OpenRouterBackend, *string, *string) {
	t.Helper()
	requestPath := ""
	requestBody := ""
	standInServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		document, errorValue := io.ReadAll(request.Body)
		if errorValue != nil {
			t.Error(errorValue)
			return
		}
		requestPath = request.URL.Path
		requestBody = string(document)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(statusCode)
		_, _ = io.WriteString(responseWriter, responseBody)
	}))
	t.Cleanup(standInServer.Close)

	keyPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(keyPath, []byte("test-key"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := OpenRouterBackend{
		KeyPath:    keyPath,
		BaseURL:    standInServer.URL + "/api/v1/chat/completions",
		ModelName:  modelladder.PrimaryModel,
		HTTPClient: standInServer.Client(),
	}
	return backend, &requestPath, &requestBody
}

func decisionsProbeRequest() DecisionsRequest {
	return DecisionsRequest{
		State: json.RawMessage(`{"newestMessage":{"sender":"이샘플","text":"응 진행해"}}`),
		Questions: map[string]DecisionQuestion{
			"target":        ChoiceQuestion("Who is the newest message directed at?", map[string]string{"bot": "the assistant", "human": "one named person"}),
			"shouldRespond": NoulQuestion("Should the assistant reply?", nil),
			"level":         ScoreQuestion("How much effort does this turn deserve?", []string{"low", "medium", "high"}),
		},
	}
}

func TestDecideSendsTypedQuestionsToTheAlphaDecisionsRoute(t *testing.T) {
	backend, requestPath, requestBody := decisionsStandIn(t, http.StatusOK, decisionsAnswerDocument)

	if _, errorValue := backend.Decide(context.Background(), decisionsProbeRequest()); errorValue != nil {
		t.Fatal(errorValue)
	}

	if *requestPath != openRouterDecisionsPath {
		t.Fatalf("expected the decisions route %q, got %q", openRouterDecisionsPath, *requestPath)
	}
	expectedBody := `{"model":"` + modelladder.DecisionModel + `",` +
		`"state":{"newestMessage":{"sender":"이샘플","text":"응 진행해"}},` +
		`"questions":{` +
		`"level":{"type":"score","instructions":"How much effort does this turn deserve?","criteria":["low","medium","high"]},` +
		`"shouldRespond":{"type":"noul","instructions":"Should the assistant reply?"},` +
		`"target":{"type":"choice","instructions":"Who is the newest message directed at?","criteria":{"bot":"the assistant","human":"one named person"}}` +
		`}}`
	if *requestBody != expectedBody {
		t.Fatalf("the request went out as %s, expected %s", *requestBody, expectedBody)
	}
}

func TestDecideReadsChoiceNoulScoreAndCost(t *testing.T) {
	backend, _, _ := decisionsStandIn(t, http.StatusOK, decisionsAnswerDocument)

	response, errorValue := backend.Decide(context.Background(), decisionsProbeRequest())
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if response.Model != "typesafe/jev-1.13-20260917" || response.Provider != "TypeSafe" {
		t.Fatalf("the answering model and provider were lost: %+v", response)
	}
	choice, isChoice := response.Answers["target"].AsChoice()
	if !isChoice || choice.Choice != "bot" || choice.Probabilities["bot"] != 0.98 || choice.Confidence != 0.97 {
		t.Fatalf("the choice answer did not survive: %+v", response.Answers["target"])
	}
	noul, isNoul := response.Answers["shouldRespond"].AsNoul()
	if !isNoul || noul.Noul != 0 {
		t.Fatalf("a noul of zero is an answer, not a missing one: %+v", response.Answers["shouldRespond"])
	}
	score, isScore := response.Answers["level"].AsScore()
	if !isScore || score.Score != 1.4 || string(score.Legend) != `{"0": "low"}` {
		t.Fatalf("the score answer did not survive: %+v", response.Answers["level"])
	}
	if response.Usage.InputTokens != 2285 || response.Usage.OutputTokens != 826 || response.Usage.CostUSD != 9.597e-05 {
		t.Fatalf("usage did not survive: %+v", response.Usage)
	}
}

func TestDecideRefusesWithTheUpstreamStatusAndBody(t *testing.T) {
	backend, _, _ := decisionsStandIn(t, http.StatusNotFound, `{"error":{"message":"No endpoints found","code":404}}`)

	_, errorValue := backend.Decide(context.Background(), decisionsProbeRequest())
	if errorValue == nil {
		t.Fatal("expected a refused decisions call to fail")
	}
	if !strings.Contains(errorValue.Error(), "404") || !strings.Contains(errorValue.Error(), "No endpoints found") {
		t.Fatalf("the failure hid the upstream status or body: %v", errorValue)
	}
}

func TestDecideNeedsAStateAndAQuestion(t *testing.T) {
	backend, _, _ := decisionsStandIn(t, http.StatusOK, decisionsAnswerDocument)

	request := decisionsProbeRequest()
	request.State = nil
	if _, errorValue := backend.Decide(context.Background(), request); errorValue == nil {
		t.Fatal("expected a stateless decision request to be refused")
	}

	request = decisionsProbeRequest()
	request.Questions = nil
	if _, errorValue := backend.Decide(context.Background(), request); errorValue == nil {
		t.Fatal("expected a questionless decision request to be refused")
	}
}

func TestDecisionsRouteSharesTheHostOfTheConfiguredBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://openrouter.ai/api/v1/chat/completions": "https://openrouter.ai" + openRouterDecisionsPath,
		"https://gateway.example/api/v1":                "https://gateway.example" + openRouterDecisionsPath,
		"": originOf(modelladder.Endpoint) + openRouterDecisionsPath,
	}
	for baseURL, expectedURL := range cases {
		backend := OpenRouterBackend{BaseURL: baseURL}
		if backend.decisionsURL() != expectedURL {
			t.Fatalf("base %q reached %q, expected %q", baseURL, backend.decisionsURL(), expectedURL)
		}
	}
}
