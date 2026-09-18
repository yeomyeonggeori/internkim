package admind

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompanionDecisionsForwardToTheDecisionModelForSignedCompanionsOnly(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	received := make(chan map[string]any, 1)
	service.Configuration.CapabilitySocketPath = startDecisionCapabilityServer(t, func(request map[string]any) map[string]any {
		received <- request
		return map[string]any{"answers": map[string]any{"next_action": map[string]any{"type": "choice", "choice": "abstain", "confidence": 0.9}}}
	})
	handler := service.router()
	_, pairResult := pairTestCompanion(t, handler, keyPairCapabilityRequest{
		KeyPair:    keyPairForTest(t),
		Capability: `{"name":"computer_task","version":"1","privacyClass":"user_browser","estimatedLatency":"interactive","requiresUserPresence":false,"worksOffline":false}`,
	})
	body := `{"state":{"goal":"anything"},"questions":{"next_action":{"type":"choice","instructions":"pick","criteria":{"abstain":"stop"}}}}`

	unsignedResponse := httptest.NewRecorder()
	unsignedRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/decisions", strings.NewReader(body))
	unsignedRequest.Header.Set("X-INTERNKIM-COMPANION-ID", pairResult.CompanionID)
	unsignedRequest.Header.Set("X-INTERNKIM-COMPANION-TOKEN", pairResult.Token)
	handler.ServeHTTP(unsignedResponse, unsignedRequest)
	if unsignedResponse.Code != http.StatusForbidden {
		t.Fatalf("unsigned decision request answered %d", unsignedResponse.Code)
	}

	signedResponse := httptest.NewRecorder()
	signedRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/decisions", strings.NewReader(body))
	setCompanionHeaders(t, signedRequest, pairResult.companionPairResponse, pairResult.privateKey)
	handler.ServeHTTP(signedResponse, signedRequest)
	if signedResponse.Code != http.StatusOK {
		t.Fatalf("signed decision request answered %d: %s", signedResponse.Code, signedResponse.Body.String())
	}
	var answered struct {
		Answers map[string]struct {
			Choice string `json:"choice"`
		} `json:"answers"`
	}
	if errorValue := json.NewDecoder(signedResponse.Body).Decode(&answered); errorValue != nil {
		t.Fatal(errorValue)
	}
	if answered.Answers["next_action"].Choice != "abstain" {
		t.Fatalf("answers = %+v", answered.Answers)
	}
	forwarded := <-received
	if forwarded["sessionID"] != "companion:"+pairResult.CompanionID {
		t.Fatalf("forwarded session = %v", forwarded["sessionID"])
	}
	if _, hasModel := forwarded["model"]; hasModel {
		t.Fatal("the companion chose the model; the device decides that")
	}
	questions, _ := forwarded["questions"].(map[string]any)
	if _, hasQuestion := questions["next_action"]; !hasQuestion {
		t.Fatalf("forwarded questions = %v", forwarded["questions"])
	}
}

func TestCompanionDecisionsRejectAnEmptyRequest(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	handler := service.router()
	_, pairResult := pairTestCompanion(t, handler, keyPairCapabilityRequest{
		KeyPair:    keyPairForTest(t),
		Capability: `{"name":"computer_task","version":"1","privacyClass":"user_browser","estimatedLatency":"interactive","requiresUserPresence":false,"worksOffline":false}`,
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/_internkim/companion/decisions", strings.NewReader(`{"state":{"goal":"anything"}}`))
	setCompanionHeaders(t, request, pairResult.companionPairResponse, pairResult.privateKey)
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("a decision without questions answered %d", response.Code)
	}
}

func startDecisionCapabilityServer(t *testing.T, decide func(map[string]any) map[string]any) string {
	t.Helper()
	directoryPath, errorValue := os.MkdirTemp("/tmp", "ik-decide-*")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directoryPath) })
	socketPath := filepath.Join(directoryPath, "capability.sock")
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/llm/decide" {
			http.NotFound(responseWriter, request)
			return
		}
		var payload map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatal(errorValue)
		}
		_ = json.NewEncoder(responseWriter).Encode(decide(payload))
	})}
	t.Cleanup(func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
		_ = listener.Close()
	})
	go func() { _ = server.Serve(listener) }()
	return socketPath
}
