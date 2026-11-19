package capabilityd

import (
	"context"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const taskDraftAdmindPath = "/task/api/drafts"

func (service Service) invokeTaskDraftTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	return service.answerThroughAdmindAsTheRequester(ctx, request, taskDraftAdmindPath, request.Input)
}
