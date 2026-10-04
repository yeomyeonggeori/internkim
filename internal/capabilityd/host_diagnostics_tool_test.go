package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
)

func jsonResponseForCapabilityTest(statusCode int, document string) *http.Response {
	return &http.Response{StatusCode: statusCode, Body: io.NopCloser(strings.NewReader(document)), Header: http.Header{"Content-Type": {"application/json"}}}
}

func TestHostDiagnosticsCarriesTheRequesterAndKeepsTheExistingDocument(t *testing.T) {
	service := Service{HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Path != "/api/v1/diagnostics/run" || request.URL.Query().Get("taskRunID") != "run + one" {
			t.Fatalf("unexpected diagnostics request: %s %s", request.Method, request.URL)
		}
		if request.Header.Get(admindRequesterEmailHeader) != "admin@example.com" {
			t.Fatalf("the requester was lost: %v", request.Header)
		}
		return jsonResponseForCapabilityTest(http.StatusOK, `{"taskEvents":[{"body":"original evidence"}]}`), nil
	})}}
	request := capabilities.ToolInvokeRequest{ToolName: "host_diagnostics_get", Input: json.RawMessage(`{"view":"run","taskRunID":"run + one"}`), Context: capabilityprotocol.ToolInvokeContext{RequesterEmail: "admin@example.com"}}
	response, errorValue := service.invokeHostDiagnosticsTool(t.Context(), request)
	var result struct {
		Document string `json:"document"`
	}
	parseError := json.Unmarshal(response.Result, &result)
	if errorValue != nil || response.IsError || parseError != nil || result.Document != `{"taskEvents":[{"body":"original evidence"}]}` {
		t.Fatalf("diagnostic evidence was changed: %+v, %v", response, errorValue)
	}
}

func TestHostDiagnosticsPreservesTheAdministratorRefusal(t *testing.T) {
	service := Service{HTTPClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponseForCapabilityTest(http.StatusForbidden, `{"message":"only an administrator"}`), nil
	})}}
	response, errorValue := service.invokeHostDiagnosticsTool(context.Background(), capabilities.ToolInvokeRequest{ToolName: "host_diagnostics_get", Input: json.RawMessage(`{"view":"requests"}`)})
	if errorValue != nil || !response.IsError || !strings.Contains(string(response.Result), "only an administrator") {
		t.Fatalf("administrator refusal was lost: %+v, %v", response, errorValue)
	}
}
