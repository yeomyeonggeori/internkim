package capabilityd

import (
	"context"
	"encoding/json"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type personForTool struct {
	PersonID           string `json:"personID"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	MattermostUsername string `json:"mattermostUsername,omitempty"`
	Mention            string `json:"mention,omitempty"`
}

func (service Service) invokePersonList(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	summary, errorValue := service.fetchFlowAllTasks(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	people := peopleForTool(summary.Members)
	result, errorValue := json.Marshal(map[string]any{"count": len(people), "people": people})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponse(request.ToolName, "ok", result)
}

func peopleForTool(members []flowMemberForTool) []personForTool {
	people := make([]personForTool, 0, len(members))
	for _, member := range members {
		people = append(people, personForTool{
			PersonID:           member.ID,
			Name:               member.Name,
			Email:              member.Email,
			MattermostUsername: member.MattermostUsername,
			Mention:            flowTaskAddMention(member),
		})
	}
	return people
}
