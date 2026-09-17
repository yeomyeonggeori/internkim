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
	answer, errorValue := client.InvokeRecordTool(ctx, requesterEmail, "task_list", "invoke",
		json.RawMessage(`{"limit":1}`))
	if errorValue != nil {
		return TaskLabels{}, errorValue
	}
	if answer.Status < 200 || answer.Status >= 300 {
		return TaskLabels{}, fmt.Errorf("the record refused task_list with %d: %s", answer.Status, string(answer.Body))
	}
	var envelope struct {
		Result struct {
			RegisteredLabels struct {
				Businesses []registeredLabel `json:"businesses"`
				Types      []registeredLabel `json:"types"`
			} `json:"registeredLabels"`
		} `json:"result"`
	}
	if errorValue := json.Unmarshal(answer.Body, &envelope); errorValue != nil {
		return TaskLabels{}, fmt.Errorf("the record's task_list answer could not be read: %w", errorValue)
	}
	return TaskLabels{
		Businesses: labelNames(envelope.Result.RegisteredLabels.Businesses),
		Types:      labelNames(envelope.Result.RegisteredLabels.Types),
	}, nil
}

type registeredLabel struct {
	Name string `json:"name"`
}

func labelNames(labels []registeredLabel) []string {
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		names = append(names, label.Name)
	}
	return names
}
