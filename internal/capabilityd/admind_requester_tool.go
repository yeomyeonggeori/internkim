package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

var admindToolOrigin = capabilityResponseOrigin{Provider: "internkim", SelectedBackend: "device"}

func (service Service) answerThroughAdmindAsTheRequester(ctx context.Context, request capabilities.ToolInvokeRequest, admindPath string, body []byte) (capabilities.ToolInvokeResponse, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, admindRequesterURL(admindPath), bytes.NewReader(body))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	return service.answerAdmindRequesterTool(request, httpRequest, nil)
}

func (service Service) answerAdmindRequesterTool(request capabilities.ToolInvokeRequest, httpRequest *http.Request, adaptResult func(json.RawMessage) (json.RawMessage, error)) (capabilities.ToolInvokeResponse, error) {
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
		return capabilityToolFailure(admindToolOrigin, request.ToolName, response.StatusCode, json.RawMessage(result)), nil
	}
	if !json.Valid(result) {
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("%s returned invalid JSON", request.ToolName)
	}
	if adaptResult != nil {
		result, errorValue = adaptResult(result)
		if errorValue != nil {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
	}
	return capabilitySuccessResponseFrom(request.ToolName, "ok", json.RawMessage(result), admindToolOrigin)
}
