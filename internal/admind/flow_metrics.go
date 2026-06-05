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
		if task.Status == "완료" {
			metrics.CompletedTasks++
		}
		if task.Status == "요청" {
			metrics.RequestedTasks++
		}
		if task.Status == "일시정지" {
			metrics.PausedTasks++
		}
		if task.Status == "중단" {
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
	switch task.Status {
	case "완료":
		return distance
	case "진행":
		return distance / 2
	case "요청", "예정", "일시정지":
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
