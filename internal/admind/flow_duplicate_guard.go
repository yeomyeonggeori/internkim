package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (service *Service) findQuickFlowTaskDuplicate(ctx context.Context, task flowTask, members []flowMember) (flowTask, string, bool, error) {
	existingTasks, errorValue := service.readFlowTasks(ctx, task.WeekCode, members)
	if errorValue != nil {
		return flowTask{}, "", false, errorValue
	}
	sameDateTasks := flowTasksWithMatchingOwnerAndDates(existingTasks, task)
	if len(sameDateTasks) == 0 {
		return flowTask{}, "", false, nil
	}
	decision, errorValue := service.decideFlowTaskDuplicate(ctx, task, sameDateTasks)
	if errorValue != nil {
		return flowTask{}, "", false, errorValue
	}
	if !decision.IsDuplicate {
		return flowTask{}, "", false, nil
	}
	duplicateTask, found := flowTaskByID(sameDateTasks, decision.DuplicateTaskID)
	if !found {
		duplicateTask = sameDateTasks[0]
	}
	return duplicateTask, decision.Reason, true, nil
}

func (service *Service) decideFlowTaskDuplicate(ctx context.Context, task flowTask, existingTasks []flowTask) (flowDuplicateDecision, error) {
	requestDocument, errorValue := json.Marshal(flowDuplicateLLMRequest(task, existingTasks))
	if errorValue != nil {
		return flowDuplicateDecision{}, errorValue
	}
	responseDocument, errorValue := service.callCapabilityStructuredLLM(ctx, requestDocument)
	if errorValue != nil {
		return flowDuplicateDecision{}, errorValue
	}
	var response capabilityLLMResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return flowDuplicateDecision{}, errorValue
	}
	var decision flowDuplicateDecision
	if errorValue := json.Unmarshal([]byte(response.Content), &decision); errorValue != nil {
		return flowDuplicateDecision{}, fmt.Errorf("flow duplicate guard returned invalid JSON: %w", errorValue)
	}
	decision.DuplicateTaskID = strings.TrimSpace(decision.DuplicateTaskID)
	decision.Reason = strings.TrimSpace(decision.Reason)
	return decision, nil
}

func flowDuplicateLLMRequest(task flowTask, existingTasks []flowTask) map[string]any {
	return map[string]any{
		"executionMode": "remote",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a duplicate guard for a weekly work tracker. Decide whether the candidate is substantially the same real-world task as one of the existing tasks. The existing tasks already have the same start and end dates as the candidate. Treat paraphrases and translations as duplicates. Do not treat related follow-ups, separate meetings, or different deliverables as duplicates. If there is no duplicate, set duplicateTaskID to an empty string.",
			},
			{
				"role":    "user",
				"content": flowDuplicatePrompt(task, existingTasks),
			},
		},
		"structuredOutputSchema": map[string]any{
			"name":               "flow_task_duplicate_guard",
			"isStrictlyEnforced": true,
			"document": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"isDuplicate", "duplicateTaskID", "reason"},
				"properties": map[string]any{
					"isDuplicate":     map[string]any{"type": "boolean"},
					"duplicateTaskID": map[string]any{"type": "string", "enum": append([]string{""}, flowTaskIDs(existingTasks)...)},
					"reason":          map[string]any{"type": "string"},
				},
			},
		},
	}
}

func flowTasksWithMatchingOwnerAndDates(tasks []flowTask, task flowTask) []flowTask {
	result := []flowTask{}
	for _, existingTask := range tasks {
		if strings.TrimSpace(existingTask.ID) == strings.TrimSpace(task.ID) {
			continue
		}
		if strings.TrimSpace(existingTask.OwnerID) != strings.TrimSpace(task.OwnerID) {
			continue
		}
		if strings.TrimSpace(existingTask.StartDate) != strings.TrimSpace(task.StartDate) {
			continue
		}
		if strings.TrimSpace(existingTask.EndDate) != strings.TrimSpace(task.EndDate) {
			continue
		}
		result = append(result, existingTask)
	}
	return result
}

func flowTaskByID(tasks []flowTask, taskID string) (flowTask, bool) {
	for _, task := range tasks {
		if strings.TrimSpace(task.ID) == strings.TrimSpace(taskID) {
			return task, true
		}
	}
	return flowTask{}, false
}

func flowDuplicatePrompt(task flowTask, existingTasks []flowTask) string {
	lines := []string{
		"Candidate: " + flowTaskDuplicateLine(task),
		"Existing tasks with the same start and end dates:",
	}
	for _, existingTask := range existingTasks {
		lines = append(lines, "- "+flowTaskDuplicateLine(existingTask))
	}
	return strings.Join(lines, "\n")
}

func flowTaskDuplicateLine(task flowTask) string {
	return strings.Join([]string{
		"id=" + task.ID,
		"owner=" + task.OwnerName,
		"startDate=" + task.StartDate,
		"endDate=" + task.EndDate,
		"type=" + task.Type,
		"size=" + task.Size,
		"status=" + task.Status,
		"content=" + task.Content,
		"goal=" + task.Goal,
	}, " | ")
}

func flowTaskIDs(tasks []flowTask) []string {
	values := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if strings.TrimSpace(task.ID) != "" {
			values = append(values, task.ID)
		}
	}
	return values
}
