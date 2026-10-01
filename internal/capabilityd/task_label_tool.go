package capabilityd

import (
	"context"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const taskLabelAdmindPath = "/task/api/labels"

func (service Service) invokeTaskLabelTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	return service.answerThroughAdmindAsTheRequester(ctx, request, taskLabelAdmindPath, request.Input)
}
