package centralplane

import (
	"context"
	"encoding/json"
	"fmt"
)

// The business and type labels a company has registered for its tasks.
type TaskLabels struct {
	Businesses []string
	Types      []string
}

func (client *Client) TaskLabels(ctx context.Context, requesterEmail string) (TaskLabels, error) {
	answer, errorValue := client.InvokeRecordTool(ctx, RecordToolCall{
		RequesterEmail: requesterEmail,
		ToolName:       "task_list",
		Verb:           "invoke",
		Input:          json.RawMessage(`{"limit":1}`),
	})
	if errorValue != nil {
		return TaskLabels{}, errorValue
	}
	if answer.Status < 200 || answer.Status >= 300 {
		return TaskLabels{}, fmt.Errorf("the record refused task_list with %d: %s", answer.Status, string(answer.Body))
	}
	var envelope struct {
		Result struct {
			RegisteredLabels struct {
				Businesses []string `json:"businesses"`
				Types      []string `json:"types"`
			} `json:"registeredLabels"`
		} `json:"result"`
	}
	if errorValue := json.Unmarshal(answer.Body, &envelope); errorValue != nil {
		return TaskLabels{}, fmt.Errorf("the record's task_list answer could not be read: %w", errorValue)
	}
	return TaskLabels{
		Businesses: envelope.Result.RegisteredLabels.Businesses,
		Types:      envelope.Result.RegisteredLabels.Types,
	}, nil
}
