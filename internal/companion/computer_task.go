package companion

import (
	"context"
	"errors"
	"net/http"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/companion/computer"
)

type ComputerTaskRunner interface {
	Run(ctx context.Context, request computer.Request) (computer.Result, error)
}

type computerTaskInput struct {
	Goal     string           `json:"goal"`
	StartURL string           `json:"startURL"`
	Inputs   []computer.Input `json:"inputs"`
	MaxSteps int              `json:"maxSteps"`
}

func (executor Executor) executeComputerTask(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.ComputerTasks == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("computer control is not set up on this companion: Cua Driver was not found")
	}
	var input computerTaskInput
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := executor.ComputerTasks.Run(ctx, computer.Request{
		Goal:     input.Goal,
		StartURL: input.StartURL,
		Inputs:   input.Inputs,
		MaxSteps: input.MaxSteps,
	})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, result)
}

// DeviceChooser puts each decision to the paired device, which holds the
// decision model's key; the companion only relays state and questions.
type DeviceChooser struct {
	DeviceClient DeviceClient
}

func (chooser DeviceChooser) Decide(ctx context.Context, request computer.DecisionRequest) (computer.DecisionResponse, error) {
	var response computer.DecisionResponse
	endpoint := chooser.DeviceClient.State.DeviceURL + "/_internkim/companion/decisions"
	if errorValue := chooser.DeviceClient.SignedJSONRequestWithContext(ctx, http.MethodPost, endpoint, request, &response); errorValue != nil {
		return computer.DecisionResponse{}, errorValue
	}
	return response, nil
}
