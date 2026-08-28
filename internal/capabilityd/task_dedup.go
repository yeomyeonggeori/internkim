package capabilityd

import (
	"log"
	"strings"
	"time"
)

const taskAddDuplicateWindow = 10 * time.Minute

func findRecentDuplicateTask(tasks []taskForTool, ownerID string, title string, now time.Time) (taskForTool, bool) {
	for _, task := range tasks {
		if task.OwnerID != ownerID || task.Content != title {
			continue
		}
		if taskCreatedWithinDuplicateWindow(task.CreatedAt, now) {
			return task, true
		}
	}
	return taskForTool{}, false
}

func taskCreatedWithinDuplicateWindow(createdAt string, now time.Time) bool {
	createdAtTime, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(createdAt))
	if errorValue != nil {
		return false
	}
	elapsed := now.Sub(createdAtTime)
	return elapsed >= 0 && elapsed <= taskAddDuplicateWindow
}

func logTaskAddDeduplicated(taskID string, ownerID string) {
	log.Printf("task_add deduplicated: taskID=%s ownerID=%s", taskID, ownerID)
}

func mergeTaskAddInputIntoDuplicate(task taskForTool, input taskAddInput) (taskForTool, bool) {
	hasNewValues := false
	applyValue := func(target *string, value string) {
		value = strings.TrimSpace(value)
		if value != "" && value != *target {
			*target = value
			hasNewValues = true
		}
	}
	applyValue(&task.Size, input.Size)
	applyValue(&task.Status, input.Status)
	applyValue(&task.Business, input.Business)
	applyValue(&task.Type, input.Type)
	applyValue(&task.StartDate, input.StartsAt)
	applyValue(&task.EndDate, input.EndsAt)
	return task, hasNewValues
}
