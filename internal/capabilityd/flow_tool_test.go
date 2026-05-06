package capabilityd

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestMatchFlowMemberFindsNameInsidePrompt(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "lee", Name: "lee", Email: "lee@example.com"},
		{ID: "iam", Name: "iam", Email: "iam@example.com"},
	}
	memberID := matchFlowMember("lee에게 10분 회의 추가해줘", members)
	if memberID != "lee" {
		t.Fatalf("memberID = %q", memberID)
	}
}

func TestFlowTaskAddPropagatesRequesterEmail(t *testing.T) {
	var summaryRequesterEmail string
	var taskRequesterEmail string
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/summary":
				summaryRequesterEmail = request.Header.Get(flowRequesterEmailHeader)
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"}]}`), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks/quick":
				taskRequesterEmail = request.Header.Get(flowRequesterEmailHeader)
				return flowToolJSONResponse(`{"id":"task-1"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	_, errorValue := service.invokeFlowTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.add",
		Input:    []byte(`{"prompt":"10분 회의"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if summaryRequesterEmail != "staff@example.com" || taskRequesterEmail != "staff@example.com" {
		t.Fatalf("requester headers summary=%q task=%q", summaryRequesterEmail, taskRequesterEmail)
	}
}

func flowToolJSONResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}
