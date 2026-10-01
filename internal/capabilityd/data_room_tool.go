package capabilityd

import (
	"context"
	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func (service Service) invokeDataRoomClassification(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	return service.answerThroughAdmindAsTheRequester(ctx, request, "/data-room/api/classify", request.Input)
}
