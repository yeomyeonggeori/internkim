package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const flowDefinitionsStateBody = `{
	"definitions": {
		"categories": ["김인턴", "여명거리", "기본소득"],
		"categoryColors": {"김인턴": "#db3333", "기본소득": "#475569"},
		"types": ["기능", "수정"],
		"typeColors": {"기능": "#216fe4"},
		"sizes": [{"name": "XS"}, {"name": "S"}, {"name": "M"}]
	},
	"statusOptions": ["예정", "진행", "완료"]
}`

func flowDefinitionsService(t *testing.T, body string, requesterEmail *string) Service {
	t.Helper()
	return Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet || request.URL.String() != "http://admind.local/flow/api/state" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
			if requesterEmail != nil {
				*requesterEmail = request.Header.Get(flowRequesterEmailHeader)
			}
			return flowToolJSONResponse(body), nil
		})},
	}
}

func invokeFlowDefinitions(t *testing.T, service Service) map[string]any {
	t.Helper()
	response, errorValue := service.invokeFlowTaskDefinitions(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.definitions",
		Input:    []byte(`{}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "staff@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result map[string]any
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	return result
}

func TestFlowTaskDefinitionsReturnsWorkspaceValues(t *testing.T) {
	result := invokeFlowDefinitions(t, flowDefinitionsService(t, flowDefinitionsStateBody, nil))

	businesses, isList := result["businesses"].([]any)
	if !isList || len(businesses) != 3 {
		t.Fatalf("businesses = %#v", result["businesses"])
	}
	first, _ := businesses[0].(map[string]any)
	if first["value"] != "김인턴" || first["color"] != "#db3333" {
		t.Fatalf("first business = %#v", businesses[0])
	}
	third, _ := businesses[2].(map[string]any)
	if third["value"] != "기본소득" || third["color"] != "#475569" {
		t.Fatalf("third business = %#v", businesses[2])
	}

	types, isList := result["types"].([]any)
	if !isList || len(types) != 2 {
		t.Fatalf("types = %#v", result["types"])
	}
	uncoloredType, _ := types[1].(map[string]any)
	if uncoloredType["value"] != "수정" {
		t.Fatalf("second type = %#v", types[1])
	}
	if _, hasColor := uncoloredType["color"]; hasColor {
		t.Fatalf("uncolored type must omit color: %#v", types[1])
	}

	sizes, _ := result["sizes"].([]any)
	if len(sizes) != 3 || sizes[0] != "XS" || sizes[2] != "M" {
		t.Fatalf("sizes = %#v", result["sizes"])
	}
	statuses, _ := result["statuses"].([]any)
	if len(statuses) != 3 || statuses[0] != "예정" {
		t.Fatalf("statuses = %#v", result["statuses"])
	}
}

func TestFlowTaskDefinitionsPropagatesRequesterEmail(t *testing.T) {
	var requesterEmail string
	invokeFlowDefinitions(t, flowDefinitionsService(t, flowDefinitionsStateBody, &requesterEmail))
	if requesterEmail != "staff@example.com" {
		t.Fatalf("requester header = %q", requesterEmail)
	}
}

func TestFlowTaskDefinitionsReturnsEmptyListsWhenWorkspaceHasNone(t *testing.T) {
	result := invokeFlowDefinitions(t, flowDefinitionsService(t, `{}`, nil))
	for _, field := range []string{"businesses", "types", "sizes", "statuses"} {
		values, isList := result[field].([]any)
		if !isList || len(values) != 0 {
			t.Fatalf("%s = %#v", field, result[field])
		}
	}
}

func TestFlowTaskDefinitionsRoutesThroughFlowTaskTool(t *testing.T) {
	service := flowDefinitionsService(t, flowDefinitionsStateBody, nil)
	response, errorValue := service.invokeFlowTaskTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.definitions",
		Input:    []byte(`{}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "staff@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded || response.Status != "ok" {
		t.Fatalf("outcome=%q status=%q", response.Outcome, response.Status)
	}
}
