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

type flowSummaryMemberIdentity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func flowSummaryDefinitionsSourceKey() flowSummarySourceKey {
	return flowSummarySourceKey{Kind: flowSummarySourceDefinitions, Key: "global"}
}

func flowSummaryDependencyKeysForWeek(weekCode string, weekStart time.Time) flowSummaryDependencyKeys {
	currentMonthStart := startOfMonth(weekStart)
	return flowSummaryDependencyKeys{
		RequestedWeek: flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: weekCode},
		PreviousWeek:  flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: weekCodeForDate(weekStart.AddDate(0, 0, -7))},
		CurrentMonth:  flowSummarySourceKey{Kind: flowSummarySourceMonth, Key: currentMonthStart.Format("2006-01")},
		PreviousMonth: flowSummarySourceKey{Kind: flowSummarySourceMonth, Key: currentMonthStart.AddDate(0, -1, 0).Format("2006-01")},
		Definitions:   flowSummaryDefinitionsSourceKey(),
	}
}

func flowSummaryMemberFingerprint(members []flowMember) (string, error) {
	identities := make([]flowSummaryMemberIdentity, 0, len(members))
	for _, member := range members {
		identities = append(identities, flowSummaryMemberIdentity{ID: member.ID, Name: member.Name})
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

func flowTaskSummarySourceKeys(task flowTask) []flowSummarySourceKey {
	keySet := map[flowSummarySourceKey]struct{}{}
	if weekKey, errorValue := flowTaskWeekSummarySourceKey(task); errorValue == nil {
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
		keySet[flowSummarySourceKey{Kind: flowSummarySourceMonth, Key: parsedDate.Format("2006-01")}] = struct{}{}
	}
	return sortedFlowSummarySourceKeys(keySet)
}

func flowTaskWeekSummarySourceKey(task flowTask) (flowSummarySourceKey, error) {
	weekCode := canonicalFlowSummaryWeekCode(strings.TrimSpace(task.WeekCode), flowDateNow())
	if weekCode == "" {
		return flowSummarySourceKey{}, fmt.Errorf("derive flow summary cache scope for task %q: invalid week code %q", task.ID, task.WeekCode)
	}
	return flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: weekCode}, nil
}

func flowTasksSummarySourceKeys(tasks []flowTask) []flowSummarySourceKey {
	keySet := map[flowSummarySourceKey]struct{}{}
	for _, task := range tasks {
		for _, key := range flowTaskSummarySourceKeys(task) {
			keySet[key] = struct{}{}
		}
	}
	return sortedFlowSummarySourceKeys(keySet)
}

func flowTaskBoardMoveSummarySourceKeys(previousTasks []flowTask, updates []flowTask) []flowSummarySourceKey {
	previousTasksByID := make(map[string]flowTask, len(previousTasks))
	for _, task := range previousTasks {
		previousTasksByID[strings.TrimSpace(task.ID)] = task
	}
	sourceTasks := []flowTask{}
	for _, updatedTask := range updates {
		previousTask, found := previousTasksByID[strings.TrimSpace(updatedTask.ID)]
		if !found || (previousTask.Status == updatedTask.Status && previousTask.StatusRank == updatedTask.StatusRank) {
			continue
		}
		sourceTasks = append(sourceTasks, previousTask, updatedTask)
	}
	return flowTasksSummarySourceKeys(sourceTasks)
}

func sortedFlowSummarySourceKeys(keySet map[flowSummarySourceKey]struct{}) []flowSummarySourceKey {
	keys := make([]flowSummarySourceKey, 0, len(keySet))
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
