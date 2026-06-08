package admind

import (
	"math"
	"time"
)

const flowScorePeriodCount = 5

func flowScoreDateRange(weekStart time.Time) (time.Time, time.Time) {
	weeklyStart := weekStart.AddDate(0, 0, -7*(flowScorePeriodCount-1))
	monthlyStart := startOfMonth(weekStart).AddDate(0, -(flowScorePeriodCount - 1), 0)
	if monthlyStart.Before(weeklyStart) {
		return monthlyStart, endOfMonth(weekStart)
	}
	return weeklyStart, endOfMonth(weekStart)
}

func buildFlowMemberScores(tasks []flowTask, members []flowMember, definitions flowDefinitions, weekStart time.Time) map[string]int {
	weeklyDistances := initializedMemberScorePeriods(members)
	monthlyDistances := initializedMemberScorePeriods(members)
	for _, task := range tasks {
		endDate, ok := flowCompletedTaskEndDate(task, weekStart.Location())
		if !ok {
			continue
		}
		distance := completedDistanceForTask(task, definitions)
		if distance <= 0 {
			continue
		}
		addTaskScoreDistance(weeklyDistances, flowScoreWeekIndex(weekStart, endDate), task, members, distance)
		addTaskScoreDistance(monthlyDistances, flowScoreMonthIndex(startOfMonth(weekStart), endDate), task, members, distance)
	}

	scores := map[string]int{}
	for _, member := range members {
		weeklyScore := weightedFlowScore(weeklyDistances[member.Name])
		monthlyScore := weightedFlowScore(monthlyDistances[member.Name])
		scores[member.Name] = int(math.Round((weeklyScore + monthlyScore) / 2))
	}
	return scores
}

func initializedMemberScorePeriods(members []flowMember) map[string][]int {
	values := map[string][]int{}
	for _, member := range members {
		values[member.Name] = make([]int, flowScorePeriodCount)
	}
	return values
}

func addTaskScoreDistance(periodDistances map[string][]int, index int, task flowTask, members []flowMember, distance int) {
	if index < 0 || index >= flowScorePeriodCount {
		return
	}
	for _, participantName := range flowScoreParticipantNames(task, members) {
		distances, found := periodDistances[participantName]
		if !found {
			continue
		}
		distances[index] += distance
	}
}

func flowScoreParticipantNames(task flowTask, members []flowMember) []string {
	memberNamesByID := map[string]string{}
	for _, member := range members {
		memberNamesByID[member.ID] = member.Name
	}
	names := make([]string, 0, len(task.ParticipantIDs)+len(task.ParticipantNames))
	seen := map[string]bool{}
	for _, participantID := range task.ParticipantIDs {
		name := memberNamesByID[participantID]
		if name != "" && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	for _, name := range task.ParticipantNames {
		if name != "" && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	return names
}

func flowScoreWeekIndex(weekStart time.Time, date time.Time) int {
	taskWeekStart := startOfISOWeek(date)
	return int(weekStart.Sub(taskWeekStart).Hours() / 24 / 7)
}

func flowScoreMonthIndex(monthStart time.Time, date time.Time) int {
	return (monthStart.Year()-date.Year())*12 + int(monthStart.Month()-date.Month())
}

func weightedFlowScore(distances []int) float64 {
	if len(distances) == 0 {
		return 0
	}
	weightedTotal := 0.0
	weightTotal := 0.0
	for index, weight := range flowScoreWeights() {
		if index >= len(distances) {
			break
		}
		weightedTotal += flowScoreUnit(distances[index], distances[index:]) * weight
		weightTotal += weight
	}
	if weightTotal == 0 {
		return 0
	}
	return weightedTotal / weightTotal * 100
}

func flowScoreWeights() [flowScorePeriodCount]float64 {
	return [flowScorePeriodCount]float64{1.5, 1.4, 1.3, 1.2, 1.1}
}

func flowScoreUnit(distance int, comparisonDistances []int) float64 {
	average := averageDistance(comparisonDistances)
	if average == 0 {
		return 0
	}
	return float64(distance) / average
}

func averageDistance(distances []int) float64 {
	if len(distances) == 0 {
		return 0
	}
	total := 0
	for _, distance := range distances {
		total += distance
	}
	return float64(total) / float64(len(distances))
}

func applyFlowMemberScores(members []flowMember, scores map[string]int) []flowMember {
	result := append([]flowMember(nil), members...)
	for index, member := range result {
		result[index].Score = scores[member.Name]
	}
	return result
}

func totalFlowScore(scores map[string]int) int {
	total := 0
	for _, score := range scores {
		total += score
	}
	return total
}

func endOfMonth(date time.Time) time.Time {
	return startOfMonth(date).AddDate(0, 1, -1)
}
