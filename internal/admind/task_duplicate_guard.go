package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (service *Service) findQuickTaskDuplicate(ctx context.Context, task Task, members []taskMember) (Task, string, bool, error) {
	existingTasks, errorValue := service.readTasks(ctx, task.WeekCode, members)
	if errorValue != nil {
		return Task{}, "", false, errorValue
	}
	sameDateTasks := tasksWithMatchingOwnerAndDates(existingTasks, task)
	if len(sameDateTasks) == 0 {
		return Task{}, "", false, nil
	}
	decision, errorValue := service.decideTaskDuplicate(ctx, task, sameDateTasks)
	if errorValue != nil {
		return Task{}, "", false, errorValue
	}
	if !decision.IsDuplicate {
		return Task{}, "", false, nil
	}
	duplicateTask, found := taskByID(sameDateTasks, decision.DuplicateTaskID)
	if !found {
		duplicateTask = sameDateTasks[0]
	}
	return duplicateTask, decision.Reason, true, nil
}

func (service *Service) decideTaskDuplicate(ctx context.Context, task Task, existingTasks []Task) (taskDuplicateDecision, error) {
	requestDocument, errorValue := json.Marshal(taskDuplicateLLMRequest(task, existingTasks))
	if errorValue != nil {
		return taskDuplicateDecision{}, errorValue
	}
	responseDocument, errorValue := service.callCapabilityLLM(ctx, "/v1/llm/structured", requestDocument)
	if errorValue != nil {
		return taskDuplicateDecision{}, errorValue
	}
	var response capabilityLLMResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return taskDuplicateDecision{}, errorValue
	}
	var decision taskDuplicateDecision
	if errorValue := json.Unmarshal([]byte(response.Content), &decision); errorValue != nil {
		return taskDuplicateDecision{}, fmt.Errorf("flow duplicate guard returned invalid JSON: %w", errorValue)
	}
	decision.DuplicateTaskID = strings.TrimSpace(decision.DuplicateTaskID)
	decision.Reason = strings.TrimSpace(decision.Reason)
	return decision, nil
}

func taskDuplicateLLMRequest(task Task, existingTasks []Task) map[string]any {
	return map[string]any{
		"executionMode": "remote",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a duplicate guard for a weekly work tracker. Decide whether the candidate is substantially the same real-world task as one of the existing tasks. The existing tasks already have the same start and end dates as the candidate. Treat paraphrases and translations as duplicates. Do not treat related follow-ups, separate meetings, or different deliverables as duplicates. If there is no duplicate, set duplicateTaskID to an empty string.",
			},
			{
				"role":    "user",
				"content": taskDuplicatePrompt(task, existingTasks),
			},
		},
		"structuredOutputSchema": map[string]any{
			"name":               "task_duplicate_guard",
			"isStrictlyEnforced": true,
			"document": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"isDuplicate", "duplicateTaskID", "reason"},
				"properties": map[string]any{
					"isDuplicate":     map[string]any{"type": "boolean"},
					"duplicateTaskID": map[string]any{"type": "string", "enum": append([]string{""}, taskIDs(existingTasks)...)},
					"reason":          map[string]any{"type": "string"},
				},
			},
		},
	}
}

func tasksWithMatchingOwnerAndDates(tasks []Task, task Task) []Task {
	result := []Task{}
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

func taskByID(tasks []Task, taskID string) (Task, bool) {
	for _, task := range tasks {
		if strings.TrimSpace(task.ID) == strings.TrimSpace(taskID) {
			return task, true
		}
	}
	return Task{}, false
}

func taskDuplicatePrompt(task Task, existingTasks []Task) string {
	lines := []string{
		"Candidate: " + taskDuplicateLine(task),
		"Existing tasks with the same start and end dates:",
	}
	for _, existingTask := range existingTasks {
		lines = append(lines, "- "+taskDuplicateLine(existingTask))
	}
	return strings.Join(lines, "\n")
}

func taskDuplicateLine(task Task) string {
	return strings.Join([]string{
		"id=" + task.ID,
		"owner=" + task.OwnerName,
		"startDate=" + task.StartDate,
		"endDate=" + task.EndDate,
		"type=" + task.Type,
		"size=" + task.Size,
		"status=" + task.Status,
		"content=" + task.Content,
	}, " | ")
}

func taskIDs(tasks []Task) []string {
	values := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if strings.TrimSpace(task.ID) != "" {
			values = append(values, task.ID)
		}
	}
	return values
}
