package capabilityd

import (
	"log"
	"strings"
	"time"
)

const flowTaskAddDuplicateWindow = 10 * time.Minute

func findRecentDuplicateFlowTask(tasks []flowTaskForTool, ownerID string, title string, now time.Time) (flowTaskForTool, bool) {
	for _, task := range tasks {
		if task.OwnerID != ownerID || task.Content != title {
			continue
		}
		if flowTaskCreatedWithinDuplicateWindow(task.CreatedAt, now) {
			return task, true
		}
	}
	return flowTaskForTool{}, false
}

func flowTaskCreatedWithinDuplicateWindow(createdAt string, now time.Time) bool {
	createdAtTime, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(createdAt))
	if errorValue != nil {
		return false
	}
	elapsed := now.Sub(createdAtTime)
	return elapsed >= 0 && elapsed <= flowTaskAddDuplicateWindow
}

func logFlowTaskAddDeduplicated(taskID string, ownerID string) {
	log.Printf("task_add deduplicated: taskID=%s ownerID=%s", taskID, ownerID)
}

func mergeFlowTaskAddInputIntoDuplicate(task flowTaskForTool, input flowTaskAddInput) (flowTaskForTool, bool) {
	hasNewValues := false
	applyValue := func(target *string, value string) {
		value = strings.TrimSpace(value)
		if value != "" && value != *target {
			*target = value
			hasNewValues = true
		}
	}
	applyValue(&task.Goal, input.Goal)
	applyValue(&task.Size, input.Size)
	applyValue(&task.Status, input.Status)
	applyValue(&task.StartDate, input.StartDate)
	applyValue(&task.EndDate, input.EndDate)
	return task, hasNewValues
}
