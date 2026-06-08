package admind

import "strings"

func buildFlowMetrics(tasks []flowTask, definitions flowDefinitions) flowMetrics {
	metrics := flowMetrics{
		StatusCounts:    map[string]int{},
		BusinessCounts:  map[string]int{},
		TypeCounts:      map[string]int{},
		MemberDistances: map[string]int{},
		MemberScores:    map[string]int{},
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
		distance := progressDistanceForTask(task, definitions)
		metrics.TotalDistance += distance
		metrics.TotalScore += distance
		for _, name := range task.ParticipantNames {
			metrics.MemberDistances[name] += distance
			metrics.MemberScores[name] += distance
		}
	}
	return metrics
}

func progressDistanceForTask(task flowTask, definitions flowDefinitions) int {
	distance := distanceForTaskSize(task.Size, definitions.Sizes)
	switch {
	case isFlowCompletedStatus(task.Status):
		return distance
	case isFlowInProgressStatus(task.Status):
		return distance / 2
	case isFlowRequestedStatus(task.Status), isFlowPlannedStatus(task.Status), isFlowPausedStatus(task.Status):
		return 0
	default:
		return 0
	}
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
