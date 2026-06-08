package admind

import "strings"

func buildFlowMetrics(tasks []flowTask, definitions flowDefinitions) flowMetrics {
	metrics := flowMetrics{
		StatusCounts:    map[string]int{},
		BusinessCounts:  map[string]int{},
		TypeCounts:      map[string]int{},
		MemberDistances:    map[string]int{},
		MemberScores:       map[string]int{},
		MemberScoreDetails: map[string]flowMemberScoreItem{},
	}
	for _, task := range tasks {
		metrics.TotalTasks++
		metrics.StatusCounts[task.Status]++
		if task.Business != "" {
			metrics.BusinessCounts[task.Business]++
		}
		metrics.TypeCounts[task.Type]++
		if isFlowCompletedStatus(task.Status) {
			metrics.CompletedTasks++
		}
		if isFlowRequestedStatus(task.Status) {
			metrics.RequestedTasks++
		}
		if isFlowPausedStatus(task.Status) {
			metrics.PausedTasks++
		}
		if isFlowStoppedStatus(task.Status) {
			metrics.StoppedTasks++
		}
		distance := completedDistanceForTask(task, definitions)
		metrics.TotalDistance += distance
		metrics.TotalScore += distance
		for _, name := range task.ParticipantNames {
			metrics.MemberDistances[name] += distance
			metrics.MemberScores[name] += distance
		}
	}
	return metrics
}

func completedDistanceForTask(task flowTask, definitions flowDefinitions) int {
	if _, ok := flowCompletedTaskEndDate(task, flowDateLocation()); !ok {
		return 0
	}
	return distanceForTaskSize(task.Size, definitions.Sizes)
}

func distanceForTaskSize(sizeName string, definitions []flowSizeDefinition) int {
	normalizedName := strings.ToUpper(strings.TrimSpace(sizeName))
	for _, definition := range definitions {
		if strings.EqualFold(definition.Name, normalizedName) {
			return definition.DistanceKM
		}
	}
	for _, definition := range defaultFlowSizeDefinitions() {
		if strings.EqualFold(definition.Name, normalizedName) {
			return definition.DistanceKM
		}
	}
	return 0
}
