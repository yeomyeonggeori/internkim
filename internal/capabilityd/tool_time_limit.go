package capabilityd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
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
		answered <- service.answerOrRecover(boundedContext, request, handler)
	}()
	select {
	case answer := <-answered:
		return settledAnswer(ctx, boundedContext, request, answer)
	case <-boundedContext.Done():
		return answerAtTheLimit(ctx, boundedContext, request, answered)
	}
}

func (service Service) answerOrRecover(ctx context.Context, request capabilities.ToolInvokeRequest, handler capabilityToolHandler) (answer toolAnswer) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		log.Printf("capability.tool_crashed: tool=%s panic=%v\n%s", strings.TrimSpace(request.ToolName), recovered, debug.Stack())
		answer = toolAnswer{response: toolCrashedResponse(request, recovered)}
	}()
	response, errorValue := handler(service, ctx, request)
	return toolAnswer{response: response, errorValue: errorValue}
}

func answerAtTheLimit(callerContext context.Context, boundedContext context.Context, request capabilities.ToolInvokeRequest, answered <-chan toolAnswer) (capabilities.ToolInvokeResponse, error) {
	select {
	case answer := <-answered:
		return settledAnswer(callerContext, boundedContext, request, answer)
	default:
	}
	if callerContext.Err() != nil {
		return capabilities.ToolInvokeResponse{}, callerContext.Err()
	}
	return toolTimeLimitResponse(request), nil
}

func settledAnswer(callerContext context.Context, boundedContext context.Context, request capabilities.ToolInvokeRequest, answer toolAnswer) (capabilities.ToolInvokeResponse, error) {
	if answer.endedUnfinished() && theRequestedTimeRanOut(callerContext, boundedContext) {
		return toolTimeLimitResponse(request), nil
	}
	return answer.response, answer.errorValue
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
	return capabilitydFailureResponse(toolName, "timeout", "time_limit", message)
}

func toolCrashedResponse(request capabilities.ToolInvokeRequest, recovered any) capabilities.ToolInvokeResponse {
	return capabilitydFailureResponse(request.ToolName, "tool_crashed", "handler", "the tool crashed while running: "+fmt.Sprint(recovered))
}
