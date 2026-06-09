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
	return currentFlowMemberScores(buildFlowMemberScoreDetails(tasks, members, definitions, weekStart))
}

func buildFlowMemberScoreDetails(tasks []flowTask, members []flowMember, definitions flowDefinitions, weekStart time.Time) map[string]flowMemberScoreItem {
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

	scores := map[string]flowMemberScoreItem{}
	for _, member := range members {
		key := flowMemberScoreKey(member)
		weeklyScore := weightedFlowScore(weeklyDistances[key])
		monthlyScore := weightedFlowScore(monthlyDistances[key])
		scores[key] = flowMemberScoreItem{
			WeeklyScore:  int(math.Round(weeklyScore)),
			MonthlyScore: int(math.Round(monthlyScore)),
			CurrentScore: int(math.Round((weeklyScore + monthlyScore) / 2)),
		}
	}
	return scores
}

func currentFlowMemberScores(details map[string]flowMemberScoreItem) map[string]int {
	scores := map[string]int{}
	for memberKey, detail := range details {
		scores[memberKey] = detail.CurrentScore
	}
	return scores
}

func initializedMemberScorePeriods(members []flowMember) map[string][]int {
	values := map[string][]int{}
	for _, member := range members {
		values[flowMemberScoreKey(member)] = make([]int, flowScorePeriodCount)
	}
	return values
}

func addTaskScoreDistance(periodDistances map[string][]int, index int, task flowTask, members []flowMember, distance int) {
	if index < 0 || index >= flowScorePeriodCount {
		return
	}
	for _, participantKey := range flowScoreParticipantKeys(task, members) {
		distances, found := periodDistances[participantKey]
		if !found {
			continue
		}
		distances[index] += distance
	}
}

func flowScoreParticipantKeys(task flowTask, members []flowMember) []string {
	memberKeysByID := map[string]string{}
	memberKeysByName := map[string][]string{}
	for _, member := range members {
		key := flowMemberScoreKey(member)
		if member.ID != "" {
			memberKeysByID[member.ID] = key
		}
		if member.Name != "" {
			memberKeysByName[member.Name] = append(memberKeysByName[member.Name], key)
		}
	}
	keys := make([]string, 0, len(task.ParticipantIDs)+len(task.ParticipantNames))
	seen := map[string]bool{}
	for _, participantID := range task.ParticipantIDs {
		key := memberKeysByID[participantID]
		if key != "" && !seen[key] {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	for _, name := range task.ParticipantNames {
		nameKeys := memberKeysByName[name]
		if len(nameKeys) != 1 {
			continue
		}
		key := nameKeys[0]
		if key != "" && !seen[key] {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	return keys
}

func flowMemberScoreKey(member flowMember) string {
	if member.ID != "" {
		return member.ID
	}
	return member.Name
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
	baseline := averageDistance(distances)
	weightedTotal := 0.0
	weightTotal := 0.0
	for index, weight := range flowScoreWeights() {
		if index >= len(distances) {
			break
		}
		weightedTotal += flowScoreUnit(distances[index], baseline) * weight
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

func flowScoreUnit(distance int, baseline float64) float64 {
	if baseline == 0 {
		return 0
	}
	return float64(distance) / baseline
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
		result[index].Score = scores[flowMemberScoreKey(member)]
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
