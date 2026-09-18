package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func (service Service) invokeScheduleTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input := request.Input
	if len(bytes.TrimSpace(input)) == 0 {
		input = json.RawMessage("{}")
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, admindRequesterURL("/memory/api/schedules/tool-list"), bytes.NewReader(input))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, errorValue := service.askAdmindAsTheRequester(httpRequest, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	defer response.Body.Close()
	result, errorValue := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("schedule list failed: %s", strings.TrimSpace(string(result)))
	}
	if !json.Valid(result) {
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("schedule list returned invalid JSON")
	}
	return capabilitySuccessResponse(request.ToolName, "ok", json.RawMessage(result))
}
