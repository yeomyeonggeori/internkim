package admind

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type taskSummaryMemberIdentity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func taskSummaryDefinitionsSourceKey() taskSummarySourceKey {
	return taskSummarySourceKey{Kind: taskSummarySourceDefinitions, Key: "global"}
}

func taskSummaryDependencyKeysForWeek(weekCode string, weekStart time.Time) taskSummaryDependencyKeys {
	currentMonthStart := startOfMonth(weekStart)
	return taskSummaryDependencyKeys{
		RequestedWeek: taskSummarySourceKey{Kind: taskSummarySourceWeek, Key: weekCode},
		PreviousWeek:  taskSummarySourceKey{Kind: taskSummarySourceWeek, Key: weekCodeForDate(weekStart.AddDate(0, 0, -7))},
		CurrentMonth:  taskSummarySourceKey{Kind: taskSummarySourceMonth, Key: currentMonthStart.Format("2006-01")},
		PreviousMonth: taskSummarySourceKey{Kind: taskSummarySourceMonth, Key: currentMonthStart.AddDate(0, -1, 0).Format("2006-01")},
		Definitions:   taskSummaryDefinitionsSourceKey(),
	}
}

func taskSummaryMemberFingerprint(members []taskMember) (string, error) {
	identities := make([]taskSummaryMemberIdentity, 0, len(members))
	for _, member := range members {
		identities = append(identities, taskSummaryMemberIdentity{ID: member.ID, Name: member.Name})
	}
	sort.Slice(identities, func(leftIndex int, rightIndex int) bool {
		if identities[leftIndex].ID != identities[rightIndex].ID {
			return identities[leftIndex].ID < identities[rightIndex].ID
		}
		return identities[leftIndex].Name < identities[rightIndex].Name
	})
	document, errorValue := json.Marshal(identities)
	if errorValue != nil {
		return "", fmt.Errorf("encode flow summary member fingerprint: %w", errorValue)
	}
	digest := sha256.Sum256(document)
	return hex.EncodeToString(digest[:]), nil
}

func taskSummarySourceKeys(task Task) []taskSummarySourceKey {
	keySet := map[taskSummarySourceKey]struct{}{}
	if weekKey, errorValue := taskWeekSummarySourceKey(task); errorValue == nil {
		keySet[weekKey] = struct{}{}
	}
	for _, date := range []string{task.StartDate, task.EndDate} {
		trimmedDate := strings.TrimSpace(date)
		if trimmedDate == "" {
			continue
		}
		parsedDate, errorValue := time.Parse("2006-01-02", trimmedDate)
		if errorValue != nil {
			continue
		}
		keySet[taskSummarySourceKey{Kind: taskSummarySourceMonth, Key: parsedDate.Format("2006-01")}] = struct{}{}
	}
	return sortedTaskSummarySourceKeys(keySet)
}

func taskWeekSummarySourceKey(task Task) (taskSummarySourceKey, error) {
	weekCode := canonicalTaskSummaryWeekCode(strings.TrimSpace(task.WeekCode), taskDateNow())
	if weekCode == "" {
		return taskSummarySourceKey{}, fmt.Errorf("derive flow summary cache scope for task %q: invalid week code %q", task.ID, task.WeekCode)
	}
	return taskSummarySourceKey{Kind: taskSummarySourceWeek, Key: weekCode}, nil
}

func tasksSummarySourceKeys(tasks []Task) []taskSummarySourceKey {
	keySet := map[taskSummarySourceKey]struct{}{}
	for _, task := range tasks {
		for _, key := range taskSummarySourceKeys(task) {
			keySet[key] = struct{}{}
		}
	}
	return sortedTaskSummarySourceKeys(keySet)
}

func taskBoardMoveSummarySourceKeys(previousTasks []Task, updates []Task) []taskSummarySourceKey {
	previousTasksByID := make(map[string]Task, len(previousTasks))
	for _, task := range previousTasks {
		previousTasksByID[strings.TrimSpace(task.ID)] = task
	}
	sourceTasks := []Task{}
	for _, updatedTask := range updates {
		previousTask, found := previousTasksByID[strings.TrimSpace(updatedTask.ID)]
		if !found || (previousTask.Status == updatedTask.Status && previousTask.StatusRank == updatedTask.StatusRank) {
			continue
		}
		sourceTasks = append(sourceTasks, previousTask, updatedTask)
	}
	return tasksSummarySourceKeys(sourceTasks)
}

func sortedTaskSummarySourceKeys(keySet map[taskSummarySourceKey]struct{}) []taskSummarySourceKey {
	keys := make([]taskSummarySourceKey, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(leftIndex int, rightIndex int) bool {
		if keys[leftIndex].Kind != keys[rightIndex].Kind {
			return keys[leftIndex].Kind < keys[rightIndex].Kind
		}
		return keys[leftIndex].Key < keys[rightIndex].Key
	})
	return keys
}
