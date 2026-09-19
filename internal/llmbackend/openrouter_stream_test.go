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
	"sync"
	"testing"
	"time"
)

func streamingBackend(t *testing.T, handler http.HandlerFunc) OpenRouterBackend {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	keyPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(keyPath, []byte("sk-or-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return OpenRouterBackend{KeyPath: keyPath, BaseURL: server.URL, ModelName: "a-model", ProviderSort: "throughput", HTTPClient: server.Client()}
}

func writeStreamChunk(t *testing.T, responseWriter http.ResponseWriter, chunk string) {
	t.Helper()
	if _, errorValue := responseWriter.Write([]byte("data: " + chunk + "\n\n")); errorValue != nil {
		return
	}
	responseWriter.(http.Flusher).Flush()
}

func freshServingRecord(t *testing.T) *servingRecord {
	t.Helper()
	previous := sharedServingRecord
	sharedServingRecord = newServingRecord()
	t.Cleanup(func() { sharedServingRecord = previous })
	return sharedServingRecord
}

func TestAStreamedChatIsAssembledIntoOneResponse(t *testing.T) {
	freshServingRecord(t)
	var requestBody []byte
	backend := streamingBackend(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		requestBody, _ = readAllBody(request)
		responseWriter.Header().Set("Content-Type", "text/event-stream")
		responseWriter.Write([]byte(": OPENROUTER PROCESSING\n\n"))
		writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[{"delta":{"role":"assistant","content":"Hel","reasoning":"thinking"},"finish_reason":null}]}`)
		writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[{"delta":{"content":"lo"},"finish_reason":null}]}`)
		writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"task_add","arguments":"{\"title\":"}}]},"finish_reason":null}]}`)
		writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"x\"}"}}]},"finish_reason":"tool_calls"}]}`)
		writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost":0.001}}`)
		writeStreamChunk(t, responseWriter, "[DONE]")
	})

	response, errorValue := backend.CompleteChat(context.Background(), ChatRequest{Model: "a-model", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Message.Content != "Hello" || response.FinishReason != "tool_calls" || response.UpstreamProvider != "Wafer" {
		t.Fatalf("assembled response is wrong: %+v", response)
	}
	if len(response.Message.ToolCalls) != 1 || response.Message.ToolCalls[0].Function.Arguments != `{"title":"x"}` || response.Message.ToolCalls[0].ID != "call-1" {
		t.Fatalf("tool call was not stitched from its deltas: %+v", response.Message.ToolCalls)
	}
	if response.Usage.TotalTokens != 15 || response.Usage.CostUSD != 0.001 {
		t.Fatalf("usage from the final chunk was lost: %+v", response.Usage)
	}
	document := documentOf(t, requestBody)
	if document["stream"] != true || document["usage"].(map[string]any)["include"] != true {
		t.Fatalf("the request must stream and ask for usage: %s", requestBody)
	}
	if _, isNamed := document["tool_choice"]; isNamed {
		t.Fatalf("a request without a tool choice must not invent one: %s", requestBody)
	}
}

func TestASlowStreamIsHedgedWithoutItsProviderAndTheHedgeAnswers(t *testing.T) {
	record := freshServingRecord(t)
	for index := 0; index < servingJudgementMinimumSamples; index++ {
		record.recordSample("a-model", servingSample{Provider: "BaseTen", Duration: 20 * time.Millisecond, CharactersPerSecond: 1000})
	}
	var mutex sync.Mutex
	requestBodies := [][]byte{}
	firstCancelled := make(chan struct{}, 1)
	backend := streamingBackend(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		body, _ := readAllBody(request)
		mutex.Lock()
		requestBodies = append(requestBodies, body)
		attempt := len(requestBodies)
		mutex.Unlock()
		responseWriter.Header().Set("Content-Type", "text/event-stream")
		if attempt == 1 {
			writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[{"delta":{"role":"assistant","content":"h"},"finish_reason":null}]}`)
			<-request.Context().Done()
			firstCancelled <- struct{}{}
			return
		}
		writeStreamChunk(t, responseWriter, `{"provider":"BaseTen","choices":[{"delta":{"role":"assistant","content":"quick answer"},"finish_reason":"stop"}]}`)
		writeStreamChunk(t, responseWriter, "[DONE]")
	})

	response, errorValue := backend.CompleteChat(context.Background(), ChatRequest{Model: "a-model", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.UpstreamProvider != "BaseTen" || !response.UsedFallback || !strings.Contains(response.FallbackReason, "Wafer") {
		t.Fatalf("the hedge should have answered and said why: %+v", response)
	}
	select {
	case <-firstCancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("the slow first ask is let go once the hedge has answered")
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

func TestASlowFirstAskThatFinishesBeforeItsHedgeIsTheAnswer(t *testing.T) {
	record := freshServingRecord(t)
	for index := 0; index < servingJudgementMinimumSamples; index++ {
		record.recordSample("a-model", servingSample{Provider: "BaseTen", Duration: 20 * time.Millisecond, CharactersPerSecond: 1000})
	}
	var mutex sync.Mutex
	attempts := 0
	backend := streamingBackend(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		mutex.Lock()
		attempts++
		attempt := attempts
		mutex.Unlock()
		responseWriter.Header().Set("Content-Type", "text/event-stream")
		writeStreamChunk(t, responseWriter, `{"provider":"StreamLake","choices":[{"delta":{"role":"assistant","content":"s"},"finish_reason":null}]}`)
		if attempt == 2 {
			<-request.Context().Done()
			return
		}
		select {
		case <-request.Context().Done():
			return
		case <-time.After(3 * servingWatchInterval):
		}
		writeStreamChunk(t, responseWriter, `{"provider":"StreamLake","choices":[{"delta":{"content":"low but done"},"finish_reason":"stop"}]}`)
		writeStreamChunk(t, responseWriter, "[DONE]")
	})

	response, errorValue := backend.CompleteChat(context.Background(), ChatRequest{Model: "a-model", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})

	if errorValue != nil {
		t.Fatalf("a slow ask that finishes is still the answer: %v", errorValue)
	}
	if response.Message.Content != "slow but done" || response.UsedFallback {
		t.Fatalf("the first ask won, so nothing fell back: %+v", response)
	}
	if slugs := record.ignoredProviders("a-model"); len(slugs) != 0 {
		t.Fatalf("a provider that answered is not cut: %v", slugs)
	}
}

func TestAClientThatGivesUpMidStreamRemembersTheProvider(t *testing.T) {
	record := freshServingRecord(t)
	backend := streamingBackend(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "text/event-stream")
		writeStreamChunk(t, responseWriter, `{"provider":"Wafer","choices":[{"delta":{"role":"assistant","content":"h"},"finish_reason":null}]}`)
		<-request.Context().Done()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, errorValue := backend.CompleteChat(ctx, ChatRequest{Model: "a-model", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})

	if errorValue == nil {
		t.Fatal("the caller's deadline ends the stream")
	}
	if slugs := record.ignoredProviders("a-model"); len(slugs) != 1 || slugs[0] != "wafer" {
		t.Fatalf("a stream the caller abandoned counts against its provider: %v", slugs)
	}
}

func TestAWholeJSONAnswerIsStillRead(t *testing.T) {
	freshServingRecord(t)
	backend := streamingBackend(t, func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"provider":"Modal","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"whole"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	})

	response, errorValue := backend.CompleteChat(context.Background(), ChatRequest{Model: "a-model", Messages: []ChatMessage{{Role: "user", Content: "hi"}}})

	if errorValue != nil || response.Message.Content != "whole" || response.UpstreamProvider != "Modal" {
		t.Fatalf("a gateway that does not stream still answers: %v %+v", errorValue, response)
	}
}

func TestServingExpectationJudgesSlowOnlyAfterItsDelay(t *testing.T) {
	expectation := servingExpectation{MedianDuration: time.Second, MedianCharactersPerSecond: 400}
	if expectation.judgesSlow(1500*time.Millisecond, 0) {
		t.Fatal("before twice the median duration nothing is judged")
	}
	if !expectation.judgesSlow(2*time.Second, 300) {
		t.Fatal("150 characters/s against a 400 median is slow")
	}
	if expectation.judgesSlow(2*time.Second, 500) {
		t.Fatal("250 characters/s is not below half the median")
	}
}

func TestProviderSlugMatchesOpenRouterEndpointTags(t *testing.T) {
	for name, slug := range map[string]string{"Wafer": "wafer", "BaseTen": "baseten", "Sail Research": "sail-research", "Z.AI": "z-ai", "Io Net": "io-net", "": ""} {
		if providerSlug(name) != slug {
			t.Fatalf("%q -> %q, expected %q", name, providerSlug(name), slug)
		}
	}
}

func TestACutIsForgottenAfterOpenRouterOwnWindow(t *testing.T) {
	record := newServingRecord()
	now := time.Now()
	record.now = func() time.Time { return now }
	record.noteCut("a-model", "Wafer")
	if slugs := record.ignoredProviders("a-model"); len(slugs) != 1 {
		t.Fatalf("expected the cut to be remembered, got %v", slugs)
	}
	now = now.Add(servingCutMemory + time.Second)
	if slugs := record.ignoredProviders("a-model"); len(slugs) != 0 {
		t.Fatalf("expected the cut to expire, got %v", slugs)
	}
}

func readAllBody(request *http.Request) ([]byte, error) {
	defer request.Body.Close()
	return io.ReadAll(request.Body)
}
