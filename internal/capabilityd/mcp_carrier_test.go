package capabilityd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func askTheCarrier(t *testing.T, service Service, message string) (int, map[string]json.RawMessage) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(message))
	request.Header.Set("Content-Type", "application/json")
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code == http.StatusAccepted || responseRecorder.Body.Len() == 0 {
		return responseRecorder.Code, nil
	}
	var answered map[string]json.RawMessage
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &answered); errorValue != nil {
		t.Fatalf("the carrier answered %q, which is not a JSON-RPC message: %v", responseRecorder.Body.String(), errorValue)
	}
	return responseRecorder.Code, answered
}

func TestCarrierAnswersTheHandshakeItself(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: admindOnLoopbackThatFailsTheTest(t)}}

	_, answered := askTheCarrier(t, service,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"blueclaw","version":"1"}}}`)

	var result struct {
		ProtocolVersion string          `json:"protocolVersion"`
		Capabilities    json.RawMessage `json:"capabilities"`
	}
	if errorValue := json.Unmarshal(answered["result"], &result); errorValue != nil {
		t.Fatalf("initialize answered %s", answered["result"])
	}
	if result.ProtocolVersion != "2025-06-18" {
		t.Fatalf("the carrier negotiated %q", result.ProtocolVersion)
	}
	if !strings.Contains(string(result.Capabilities), "tools") {
		t.Fatalf("the carrier offered %s", result.Capabilities)
	}
	if string(answered["id"]) != "1" {
		t.Fatalf("the answer carried id %s", answered["id"])
	}
}

func TestCarrierAcceptsTheInitializedNotification(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: admindOnLoopbackThatFailsTheTest(t)}}

	status, _ := askTheCarrier(t, service, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if status != http.StatusAccepted {
		t.Fatalf("the notification answered %d", status)
	}
}

func TestCarrierForwardsToolMessagesAsTheRequesterOnMeta(t *testing.T) {
	var reachedPath, reachedRequester, reachedBody string
	socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		reachedPath = request.URL.Path
		reachedRequester = request.Header.Get(admindRequesterEmailHeader)
		reachedBody = string(body)
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"jsonrpc":"2.0","id":7,"result":{"tools":[{"name":"task_list"}]}}`))
	}))
	service := Service{Configuration: Configuration{
		AdmindBaseURL:    admindOnLoopbackThatFailsTheTest(t),
		AdmindSocketPath: socketPath,
	}}

	message := `{"jsonrpc":"2.0","id":7,"method":"tools/list","params":{"_meta":{"kim.intern/requester":"Member@Example.com"}}}`
	_, answered := askTheCarrier(t, service, message)

	if reachedPath != "/record/api/mcp" {
		t.Fatalf("the message went to %q", reachedPath)
	}
	if reachedRequester != "member@example.com" {
		t.Fatalf("admind was asked as %q", reachedRequester)
	}
	if reachedBody != message {
		t.Fatalf("the message arrived as %q", reachedBody)
	}
	if !strings.Contains(string(answered["result"]), "task_list") {
		t.Fatalf("the carrier answered %s", answered["result"])
	}
}

func TestCarrierRefusesAToolMessageThatNamesNobody(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: admindOnLoopbackThatFailsTheTest(t)}}

	_, answered := askTheCarrier(t, service, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"task_list","arguments":{}}}`)

	var refusal struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if errorValue := json.Unmarshal(answered["error"], &refusal); errorValue != nil {
		t.Fatalf("a message naming nobody answered %s", answered["result"])
	}
	if refusal.Code != mcpProtocolErrorParams || !strings.Contains(refusal.Message, mcpRequesterMetaKey) {
		t.Fatalf("the refusal was %+v", refusal)
	}
}

func TestCarrierServesToolsAndNothingElse(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: admindOnLoopbackThatFailsTheTest(t)}}

	_, answered := askTheCarrier(t, service, `{"jsonrpc":"2.0","id":3,"method":"resources/list","params":{}}`)

	var refusal struct {
		Code int `json:"code"`
	}
	if errorValue := json.Unmarshal(answered["error"], &refusal); errorValue != nil {
		t.Fatalf("resources/list answered %s", answered["result"])
	}
	if refusal.Code != mcpProtocolErrorMethod {
		t.Fatalf("the refusal carried code %d", refusal.Code)
	}
}
