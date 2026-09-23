package llmbackend

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTheLastAttemptToAnswerIsTheExchange(t *testing.T) {
	startedAt := time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC)
	snapshot := ExchangeCaptureSnapshot{Attempts: []ExchangeCaptureAttempt{
		{StartedAt: startedAt, DurationMS: 100, Method: http.MethodPost, URL: "https://router.example.com/v1/chat/completions", Status: http.StatusOK, RequestBody: `{"try":1}`, ResponseBody: `{"choices":[{"message":{"content":""}}]}`},
		{StartedAt: startedAt.Add(time.Second), DurationMS: 100, Method: http.MethodPost, URL: "https://router.example.com/v1/chat/completions", Status: http.StatusBadGateway, RequestBody: `{"try":2}`, ResponseBody: `{"error":"busy"}`},
		{StartedAt: startedAt.Add(2 * time.Second), DurationMS: 100, Method: http.MethodPost, URL: "https://router.example.com/v1/chat/completions", Status: http.StatusOK, RequestBody: `{"try":3}`, ResponseBody: `{"choices":[{"message":{"content":"ok"}}]}`},
		{StartedAt: startedAt.Add(2 * time.Second), DurationMS: 900, Method: http.MethodPost, URL: "https://router.example.com/v1/chat/completions", Status: http.StatusOK, RequestBody: `{"hedge":true}`, ResponseBody: `data: {"choices":[`, ErrorCategory: "response_body"},
	}}

	exchange, isAnswered := snapshot.AnsweringExchange()

	if !isAnswered || exchange.Request != `{"try":3}` || !strings.Contains(exchange.Response, `"ok"`) {
		t.Fatalf("expected the retry that answered, not an earlier empty reply or a cancelled hedge, got %s -> %s", exchange.Request, exchange.Response)
	}
}

func TestAStreamedAnswerIsKeptAsTheCompletionItAssembles(t *testing.T) {
	stream := ": OPENROUTER PROCESSING\n\n" +
		`data: {"provider":"Example","choices":[{"index":0,"delta":{"role":"assistant","content":"안"}}]}` + "\n\n" +
		`data: {"provider":"Example","choices":[{"index":0,"delta":{"content":"녕"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}` + "\n\n" +
		"data: [DONE]\n\n"
	snapshot := ExchangeCaptureSnapshot{Attempts: []ExchangeCaptureAttempt{
		{Method: http.MethodPost, URL: "https://router.example.com/v1/chat/completions", Status: http.StatusOK, RequestBody: `{"stream":true}`, ResponseBody: stream},
	}}

	exchange, _ := snapshot.AnsweringExchange()

	response := exchange.Response
	if !strings.Contains(response, `"provider":"Example"`) || !strings.Contains(response, `"안녕"`) || strings.Contains(response, "data:") {
		t.Fatalf("expected one completion naming the provider that served it, got %s", response)
	}
}

func TestACallThatNeverAnsweredHasNoExchange(t *testing.T) {
	snapshot := ExchangeCaptureSnapshot{Attempts: []ExchangeCaptureAttempt{
		{Method: http.MethodPost, Status: http.StatusTooManyRequests, ResponseBody: `{"error":"rate"}`},
		{Method: http.MethodPost, ErrorCategory: "transport"},
	}}

	if _, isAnswered := snapshot.AnsweringExchange(); isAnswered {
		t.Fatal("expected no exchange from a call nothing answered")
	}
}

func TestAnAttemptsEndpointDropsItsQuery(t *testing.T) {
	client, capture := NewExchangeCapture(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header), Request: request}, nil
	})})

	client.Post("https://user:secret@model.example.com/v1/chat?key=secret", "application/json", strings.NewReader(`{}`))

	if endpoint := capture.Snapshot().Attempts[0].URL; endpoint != "https://model.example.com/v1/chat" {
		t.Fatalf("expected the endpoint without credentials, got %q", endpoint)
	}
}
