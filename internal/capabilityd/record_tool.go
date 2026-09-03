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
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

// These tools live in the record, so this carries the call to admind, which
// runs it on the plane as the person who asked. Nothing is decided here.
func (service Service) invokeRecordTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	toolName := strings.TrimSpace(request.ToolName)
	if !theRecordAnswers(toolName) {
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("record tool is not configured: %s", toolName)
	}

	answer, status, errorValue := service.askTheRecord(ctx, toolName, "invoke", request.Input, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if status >= http.StatusBadRequest {
		return recordToolFailure(toolName, status, answer), nil
	}

	var answered struct {
		Result json.RawMessage `json:"result"`
	}
	if errorValue := json.Unmarshal(answer, &answered); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponseFrom(
		toolName,
		"ok",
		answered.Result,
		capabilityResponseOrigin{Provider: "internkim", SelectedBackend: "record"},
	)
}

func (service Service) askTheRecord(
	ctx context.Context,
	toolName string,
	verb string,
	input json.RawMessage,
	requesterEmail string,
) (json.RawMessage, int, error) {
	body := input
	if len(bytes.TrimSpace(body)) == 0 {
		body = json.RawMessage("{}")
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost,
		admindRequesterURL("/record/api/tools/"+toolName+"/"+verb), bytes.NewReader(body))
	if errorValue != nil {
		return nil, 0, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := service.askAdmindAsTheRequester(httpRequest, requesterEmail)
	if errorValue != nil {
		return nil, 0, errorValue
	}
	defer httpResponse.Body.Close()
	answer, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return nil, 0, errorValue
	}
	return json.RawMessage(answer), httpResponse.StatusCode, nil
}

// The record says why it refused in its own words, and a hint it could not
// resolve comes back with the candidates to choose from. Both are what the
// model needs, so they are carried rather than summarised.
func recordToolFailure(toolName string, status int, answer json.RawMessage) capabilities.ToolInvokeResponse {
	var refusal struct {
		Error        string `json:"error"`
		ErrorCode    string `json:"errorCode"`
		FailureStage string `json:"failureStage"`
		Retryable    *bool  `json:"retryable"`
		SafeRetry    bool   `json:"safeRetry"`
	}
	json.Unmarshal(answer, &refusal)
	message := strings.TrimSpace(refusal.Error)
	if message == "" {
		message = strings.TrimSpace(string(answer))
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "record",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         message,
		IsError:         true,
		Message:         message,
		ErrorCode:       firstNonEmpty(refusal.ErrorCode, recordToolErrorCode(status)),
		FailureStage:    firstNonEmpty(refusal.FailureStage, recordToolFailureStage(status)),
		Retryable:       recordToolRetryable(refusal.Retryable, status),
		SafeRetry:       refusal.SafeRetry,
		Result:          answer,
	}
}

func recordToolRetryable(saidByTheRecord *bool, status int) bool {
	if saidByTheRecord != nil {
		return *saidByTheRecord
	}
	return status == http.StatusConflict
}

func recordToolErrorCode(status int) string {
	switch status {
	case http.StatusConflict:
		return "interaction_required"
	case http.StatusForbidden:
		return "not_allowed"
	default:
		return "record_refused"
	}
}

func recordToolFailureStage(status int) string {
	if status == http.StatusConflict {
		return "target_resolution"
	}
	return "execution"
}

func theRecordAnswers(toolName string) bool {
	descriptor, hasDescriptor := capabilityToolDescriptorFor(toolName)
	return hasDescriptor && descriptor.AnsweredBy == capabilityprotocol.AnsweredByRecord
}

func (service Service) previewRecordToolTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	toolName := strings.TrimSpace(request.ToolName)
	answer, status, errorValue := service.askTheRecord(ctx, toolName, "target", request.Input, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if status >= http.StatusBadRequest {
		return recordToolFailure(toolName, status, answer), nil
	}

	var previewed struct {
		Target json.RawMessage `json:"target"`
	}
	if errorValue := json.Unmarshal(answer, &previewed); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if namesNoTarget(previewed.Target) {
		return capabilityToolWithoutTargetResponse(toolName), nil
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "record",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Status:          "resolved",
		Result:          previewed.Target,
	}, nil
}

func namesNoTarget(target json.RawMessage) bool {
	trimmed := bytes.TrimSpace(target)
	return len(trimmed) == 0 || string(trimmed) == "null"
}
