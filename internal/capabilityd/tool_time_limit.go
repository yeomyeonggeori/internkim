package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

type toolAnswer struct {
	response   capabilities.ToolInvokeResponse
	errorValue error
}

func (service Service) answerWithinTheRequestedTime(ctx context.Context, request capabilities.ToolInvokeRequest, handler capabilityToolHandler) (capabilities.ToolInvokeResponse, error) {
	if request.TimeoutSecond <= 0 {
		return handler(service, ctx, request)
	}
	boundedContext, release := context.WithTimeout(ctx, time.Duration(request.TimeoutSecond)*time.Second)
	defer release()
	answered := make(chan toolAnswer, 1)
	go func() {
		response, errorValue := handler(service, boundedContext, request)
		answered <- toolAnswer{response: response, errorValue: errorValue}
	}()
	select {
	case answer := <-answered:
		if answer.endedUnfinished() && theRequestedTimeRanOut(ctx, boundedContext) {
			return toolTimeLimitResponse(request), nil
		}
		return answer.response, answer.errorValue
	case <-boundedContext.Done():
		if ctx.Err() != nil {
			return capabilities.ToolInvokeResponse{}, ctx.Err()
		}
		return toolTimeLimitResponse(request), nil
	}
}

func (answer toolAnswer) endedUnfinished() bool {
	return answer.errorValue != nil || capabilityResponseIsFailure(answer.response)
}

func theRequestedTimeRanOut(callerContext context.Context, boundedContext context.Context) bool {
	return errors.Is(boundedContext.Err(), context.DeadlineExceeded) && callerContext.Err() == nil
}

func toolTimeLimitResponse(request capabilities.ToolInvokeRequest) capabilities.ToolInvokeResponse {
	toolName := strings.TrimSpace(request.ToolName)
	message := fmt.Sprintf("%s did not finish within %d seconds, and the work it started may still have taken effect", toolName, request.TimeoutSecond)
	result, _ := json.Marshal(map[string]string{
		"errorCode":    "timeout",
		"failureStage": "time_limit",
		"message":      message,
	})
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         message,
		IsError:         true,
		Message:         message,
		ErrorCode:       "timeout",
		FailureStage:    "time_limit",
		Result:          result,
	}
}
