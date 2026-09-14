package llmbackend

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func structuredAsk(enableResponseHealing bool) StructuredRequest {
	return StructuredRequest{
		Model:                  "a-model",
		Messages:               []Message{{Role: "user", Content: "route this"}},
		StructuredOutputSchema: StructuredOutputSchema{Name: "router", Document: json.RawMessage(`{"type":"object"}`), IsStrictlyEnforced: true},
		EnableResponseHealing:  enableResponseHealing,
	}
}

func recordQuickServing(record *servingRecord) {
	for index := 0; index < servingJudgementMinimumSamples; index++ {
		record.recordSample("a-model", servingSample{Provider: "BaseTen", Duration: 20 * time.Millisecond, CharactersPerSecond: 1000})
	}
}

func TestAStreamedStructuredAnswerIsAssembledWithUsage(t *testing.T) {
	freshServingRecord(t)
	var requestBody []byte
	backend := streamingBackend(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		requestBody, _ = readAllBody(request)
		responseWriter.Header().Set("Content-Type", "text/event-stream")
		writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[{"delta":{"role":"assistant","content":"{\"answer\":"},"finish_reason":null}]}`)
		writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[{"delta":{"content":"\"yes\"}"},"finish_reason":"stop"}]}`)
		writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost":0.001}}`)
		writeStreamChunk(t, responseWriter, "[DONE]")
	})

	response, errorValue := backend.CompleteStructured(context.Background(), structuredAsk(false))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Content != `{"answer":"yes"}` || response.UpstreamProvider != "Wafer" || response.ConstraintMode != ConstraintModeOpenAIJSONSchema || response.UsedFallback {
		t.Fatalf("assembled response is wrong: %+v", response)
	}
	if response.Usage.TotalTokens != 15 || response.Usage.CostUSD != 0.001 {
		t.Fatalf("usage from the final chunk was lost: %+v", response.Usage)
	}
	document := documentOf(t, requestBody)
	if document["stream"] != true || document["usage"].(map[string]any)["include"] != true {
		t.Fatalf("the request must stream and ask for usage: %s", requestBody)
	}
	if _, isConstrained := document["response_format"]; !isConstrained {
		t.Fatalf("the schema constraint must ride along: %s", requestBody)
	}
}

func TestASlowStructuredAskIsHedgedWithoutItsProviderAndTheHedgeAnswers(t *testing.T) {
	record := freshServingRecord(t)
	recordQuickServing(record)
	var mutex sync.Mutex
	requestBodies := [][]byte{}
	backend := streamingBackend(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		body, _ := readAllBody(request)
		mutex.Lock()
		requestBodies = append(requestBodies, body)
		attempt := len(requestBodies)
		mutex.Unlock()
		responseWriter.Header().Set("Content-Type", "text/event-stream")
		if attempt == 1 {
			writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[{"delta":{"role":"assistant","content":"{"},"finish_reason":null}]}`)
			<-request.Context().Done()
			return
		}
		writeStreamChunk(t, responseWriter, `{"provider":"BaseTen","choices":[{"delta":{"role":"assistant","content":"{\"answer\":\"quick\"}"},"finish_reason":"stop"}]}`)
		writeStreamChunk(t, responseWriter, "[DONE]")
	})

	response, errorValue := backend.CompleteStructured(context.Background(), structuredAsk(false))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Content != `{"answer":"quick"}` || response.UpstreamProvider != "BaseTen" || !response.UsedFallback || !strings.Contains(response.FallbackReason, "Wafer") {
		t.Fatalf("the hedge should have answered and said why: %+v", response)
	}
	mutex.Lock()
	defer mutex.Unlock()
	if len(requestBodies) != 2 {
		t.Fatalf("expected the slow ask and one hedge, got %d requests", len(requestBodies))
	}
	if _, isIgnoring := routingOf(t, requestBodies[0])["ignore"]; isIgnoring {
		t.Fatalf("the first ask had nothing to ignore: %s", requestBodies[0])
	}
	ignored, _ := json.Marshal(routingOf(t, requestBodies[1])["ignore"])
	if string(ignored) != `["wafer"]` {
		t.Fatalf("the hedge must leave the slow provider out: %s", requestBodies[1])
	}
	if slugs := record.ignoredProviders("a-model"); len(slugs) != 1 || slugs[0] != "wafer" {
		t.Fatalf("a provider the hedge beat is remembered for the next call: %v", slugs)
	}
}

func TestASlowStructuredAskThatFinishesBeforeItsHedgeIsTheAnswer(t *testing.T) {
	record := freshServingRecord(t)
	recordQuickServing(record)
	var mutex sync.Mutex
	attempts := 0
	backend := streamingBackend(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		mutex.Lock()
		attempts++
		attempt := attempts
		mutex.Unlock()
		responseWriter.Header().Set("Content-Type", "text/event-stream")
		writeStreamChunk(t, responseWriter, `{"provider":"StreamLake","choices":[{"delta":{"role":"assistant","content":"{\"answer\":"},"finish_reason":null}]}`)
		if attempt == 2 {
			<-request.Context().Done()
			return
		}
		select {
		case <-request.Context().Done():
			return
		case <-time.After(3 * servingWatchInterval):
		}
		writeStreamChunk(t, responseWriter, `{"provider":"StreamLake","choices":[{"delta":{"content":"\"slow but done\"}"},"finish_reason":"stop"}]}`)
		writeStreamChunk(t, responseWriter, "[DONE]")
	})

	response, errorValue := backend.CompleteStructured(context.Background(), structuredAsk(false))

	if errorValue != nil {
		t.Fatalf("a slow ask that finishes is still the answer: %v", errorValue)
	}
	if response.Content != `{"answer":"slow but done"}` || response.UsedFallback {
		t.Fatalf("the first ask won, so nothing fell back: %+v", response)
	}
	if slugs := record.ignoredProviders("a-model"); len(slugs) != 0 {
		t.Fatalf("a provider that answered is not cut: %v", slugs)
	}
}

func TestAStreamedStructuredAnswerCutByItsLimitStillSaysSo(t *testing.T) {
	freshServingRecord(t)
	backend := streamingBackend(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "text/event-stream")
		writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[{"delta":{"role":"assistant","content":"{\"answer\":\"par"},"finish_reason":"length"}]}`)
		writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[],"usage":{"prompt_tokens":20,"completion_tokens":1600}}`)
		writeStreamChunk(t, responseWriter, "[DONE]")
	})

	_, errorValue := backend.CompleteStructured(context.Background(), structuredAsk(false))

	if !isStructuredOutputLimitError(errorValue) || !strings.Contains(errorValue.Error(), "completion_tokens=1600") {
		t.Fatalf("a stream cut by the output limit reports the limit and its usage: %v", errorValue)
	}
}

func TestAStreamThatEndsWithoutACompleteDocumentIsNotAnAnswer(t *testing.T) {
	freshServingRecord(t)
	backend := streamingBackend(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "text/event-stream")
		writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[{"delta":{"role":"assistant","content":"{\"answer\":"},"finish_reason":null}]}`)
		writeStreamChunk(t, responseWriter, "[DONE]")
	})

	_, errorValue := backend.CompleteStructured(context.Background(), structuredAsk(false))

	if errorValue == nil || !strings.Contains(errorValue.Error(), "structured response content was not valid json") {
		t.Fatalf("a half-written document fails validation like any other: %v", errorValue)
	}
}

func TestAResponseHealedAskIsSentWholeBecauseHealingDoesNotStream(t *testing.T) {
	freshServingRecord(t)
	var mutex sync.Mutex
	requestBodies := [][]byte{}
	backend := streamingBackend(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		body, _ := readAllBody(request)
		mutex.Lock()
		requestBodies = append(requestBodies, body)
		mutex.Unlock()
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"provider":"Modal","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"answer\":\"healed\"}"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	})

	response, errorValue := backend.CompleteStructured(context.Background(), structuredAsk(true))

	if errorValue != nil || response.Content != `{"answer":"healed"}` || response.UpstreamProvider != "Modal" || response.Usage.TotalTokens != 2 {
		t.Fatalf("a healed ask still answers whole: %v %+v", errorValue, response)
	}
	mutex.Lock()
	defer mutex.Unlock()
	if len(requestBodies) != 1 {
		t.Fatalf("a healed ask is never hedged, got %d requests", len(requestBodies))
	}
	document := documentOf(t, requestBodies[0])
	plugins, _ := json.Marshal(document["plugins"])
	if document["stream"] != false || string(plugins) != `[{"id":"response-healing"}]` {
		t.Fatalf("a healed ask must not stream and must carry the plugin: %s", requestBodies[0])
	}
}
