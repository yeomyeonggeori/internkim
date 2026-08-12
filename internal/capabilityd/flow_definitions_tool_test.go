package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
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
	"members": [{"id": "staff", "name": "Staff", "email": "staff@example.com"}],
	"tasks": [{"id": "task-1", "ownerID": "staff", "participantIDs": ["staff"], "content": "운동", "status": "예정"}],
	"statusOptions": ["예정", "진행", "완료"]
}`

func flowLabelService(t *testing.T, capturedPayload *string) Service {
	t.Helper()
	return Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
				return flowToolJSONResponse(flowDefinitionsStateBody), nil
			case request.Method == http.MethodPut && request.URL.String() == "http://admind.local/flow/api/tasks/task-1":
				*capturedPayload = readFlowRequestBody(t, request)
				return flowToolJSONResponse(echoedFlowTaskResponse(t, *capturedPayload)), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}
}

func updateFlowTaskLabel(t *testing.T, service Service, input string) capabilities.ToolInvokeResponse {
	t.Helper()
	response, errorValue := service.invokeFlowTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(input),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "staff@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return response
}

func TestFlowTaskUpdateAcceptsARegisteredLabel(t *testing.T) {
	capturedPayload := ""
	response := updateFlowTaskLabel(t, flowLabelService(t, &capturedPayload), `{"taskHint":"운동","category":"김인턴","type":"기능"}`)

	if response.IsError {
		t.Fatalf("response = %+v", response)
	}
	if !strings.Contains(capturedPayload, `"category":"김인턴"`) || !strings.Contains(capturedPayload, `"type":"기능"`) {
		t.Fatalf("payload = %s", capturedPayload)
	}
}

func TestFlowTaskUpdateReturnsRegisteredLabelsForAnUnknownOne(t *testing.T) {
	capturedPayload := ""
	response := updateFlowTaskLabel(t, flowLabelService(t, &capturedPayload), `{"taskHint":"운동","category":"없는사업"}`)

	if !response.IsError || capturedPayload != "" {
		t.Fatalf("response=%+v payload=%s", response, capturedPayload)
	}
	for _, expected := range []string{"flow_label_not_registered", `"field":"category"`, "김인턴", "여명거리", "기본소득"} {
		if !strings.Contains(string(response.Result), expected) {
			t.Fatalf("result missing %s: %s", expected, string(response.Result))
		}
	}
}

func TestFlowTaskUpdateResolvesAUniquePartialLabel(t *testing.T) {
	capturedPayload := ""
	response := updateFlowTaskLabel(t, flowLabelService(t, &capturedPayload), `{"taskHint":"운동","category":"여명"}`)

	if response.IsError {
		t.Fatalf("response = %+v", response)
	}
	if !strings.Contains(capturedPayload, `"category":"여명거리"`) {
		t.Fatalf("payload = %s", capturedPayload)
	}
}

func TestFlowTaskUpdateLeavesLabelsAloneWhenTheWorkspaceRegistersNone(t *testing.T) {
	capturedPayload := ""
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet:
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"}],"tasks":[{"id":"task-1","ownerID":"staff","participantIDs":["staff"],"content":"운동","status":"예정"}]}`), nil
			default:
				capturedPayload = readFlowRequestBody(t, request)
				return flowToolJSONResponse(echoedFlowTaskResponse(t, capturedPayload)), nil
			}
		})},
	}
	response := updateFlowTaskLabel(t, service, `{"taskHint":"운동","category":"신규사업"}`)

	if response.IsError {
		t.Fatalf("response = %+v", response)
	}
	if !strings.Contains(capturedPayload, `"category":"신규사업"`) {
		t.Fatalf("payload = %s", capturedPayload)
	}
}

func readFlowRequestBody(t *testing.T, request *http.Request) string {
	t.Helper()
	body, errorValue := io.ReadAll(request.Body)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(body)
}

func echoedFlowTaskResponse(t *testing.T, payload string) string {
	t.Helper()
	var written struct {
		Category string `json:"category"`
		Type     string `json:"type"`
		Content  string `json:"content"`
		Status   string `json:"status"`
	}
	if errorValue := json.Unmarshal([]byte(payload), &written); errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := json.Marshal(map[string]any{
		"id": "task-1", "ownerID": "staff", "participantIDs": []string{"staff"},
		"business": written.Category, "type": written.Type,
		"content": written.Content, "status": written.Status,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(document)
}
