// Package openroutertest stands in for OpenRouter at the HTTP boundary, which
// is the only place capabilityd names it: every model-using tool reaches it
// through Configuration.OpenRouterBaseURL. Faking the boundary rather than the
// tool keeps the prompt, the schema and the parsing under test, so a change to
// any of them still fails.
package openroutertest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

type Call struct {
	Model    string
	Body     map[string]any
	Messages []map[string]any
}

type Fake struct {
	server *httptest.Server

	mutex  sync.Mutex
	calls  []Call
	answer func(Call) string
}

// The model named by INTERNKIM_GATE_MODEL, when there is one, sends the gate to
// the real OpenRouter instead. A caller reads NamedModel to decide.
const ModelEnvironmentName = "INTERNKIM_GATE_MODEL"

func Start() *Fake {
	fake := &Fake{answer: func(Call) string { return "ok" }}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.serve))
	return fake
}

func (fake *Fake) BaseURL() string { return fake.server.URL }

func (fake *Fake) Close() { fake.server.Close() }

// Answers replaces what the fake says. The function sees the call, so a test
// can answer differently per model or per prompt without a second server.
func (fake *Fake) Answers(answer func(Call) string) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.answer = answer
}

func (fake *Fake) Calls() []Call {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return append([]Call(nil), fake.calls...)
}

func (fake *Fake) serve(responseWriter http.ResponseWriter, request *http.Request) {
	body, _ := io.ReadAll(request.Body)
	call := Call{}
	if errorValue := json.Unmarshal(body, &call.Body); errorValue == nil {
		if model, held := call.Body["model"].(string); held {
			call.Model = model
		}
		if messages, held := call.Body["messages"].([]any); held {
			for _, message := range messages {
				if shaped, held := message.(map[string]any); held {
					call.Messages = append(call.Messages, shaped)
				}
			}
		}
	}

	fake.mutex.Lock()
	fake.calls = append(fake.calls, call)
	answer := fake.answer
	fake.mutex.Unlock()

	responseWriter.Header().Set("Content-Type", "application/json")
	json.NewEncoder(responseWriter).Encode(map[string]any{
		"id":      "gate-1",
		"model":   call.Model,
		"choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": answer(call)}}},
		"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	})
}

// AnswersJSON is the common case: a tool that asks for structured output gets a
// document back rather than prose.
func AnswersJSON(document any) func(Call) string {
	return func(Call) string {
		encoded, _ := json.Marshal(document)
		return string(encoded)
	}
}

func (fake *Fake) AskedFor(fragment string) bool {
	for _, call := range fake.Calls() {
		for _, message := range call.Messages {
			if content, held := message["content"].(string); held && strings.Contains(content, fragment) {
				return true
			}
		}
	}
	return false
}
