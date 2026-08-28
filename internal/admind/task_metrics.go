package admind

import "strings"

func buildTaskMetrics(tasks []Task, definitions taskDefinitions) taskMetrics {
	metrics := taskMetrics{
		StatusCounts:       map[string]int{},
		BusinessCounts:     map[string]int{},
		TypeCounts:         map[string]int{},
		MemberDistances:    map[string]int{},
		MemberScores:       map[string]int{},
		MemberScoreDetails: map[string]taskMemberScoreItem{},
	}
	for _, task := range tasks {
		metrics.TotalTasks++
		metrics.StatusCounts[task.Status]++
		if task.Business != "" {
			metrics.BusinessCounts[task.Business]++
		}
		metrics.TypeCounts[task.Type]++
		if isTaskCompletedStatus(task.Status) {
			metrics.CompletedTasks++
		}
		if isTaskRequestedStatus(task.Status) {
			metrics.RequestedTasks++
		}
		if isTaskPausedStatus(task.Status) {
			metrics.PausedTasks++
		}
		if isTaskStoppedStatus(task.Status) {
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

func completedDistanceForTask(task Task, definitions taskDefinitions) int {
	if _, ok := taskCompletedTaskEndDate(task, taskDateLocation()); !ok {
		return 0
	}
	return distanceForTaskSize(task.Size, definitions.Sizes)
}

func distanceForTaskSize(sizeName string, definitions []taskSizeDefinition) int {
	normalizedName := strings.ToUpper(strings.TrimSpace(sizeName))
	for _, definition := range definitions {
		if strings.EqualFold(definition.Name, normalizedName) {
			return definition.DistanceKM
		}
	}
	for _, definition := range defaultTaskSizeDefinitions() {
		if strings.EqualFold(definition.Name, normalizedName) {
			return definition.DistanceKM
		}
	}
	return 0
}
