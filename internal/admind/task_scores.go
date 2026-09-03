package admind

import (
	"math"
	"time"
)

const taskScorePeriodCount = 5

func taskScoreDateRange(weekStart time.Time) (time.Time, time.Time) {
	weeklyStart := weekStart.AddDate(0, 0, -7*(taskScorePeriodCount-1))
	monthlyStart := startOfMonth(weekStart).AddDate(0, -(taskScorePeriodCount - 1), 0)
	if monthlyStart.Before(weeklyStart) {
		return monthlyStart, endOfMonth(weekStart)
	}
	return weeklyStart, endOfMonth(weekStart)
}

func buildTaskMemberScoreDetails(tasks []Task, members []taskMember, definitions taskDefinitions, weekStart time.Time) map[string]taskMemberScoreItem {
	weeklyDistances := initializedMemberScorePeriods(members)
	monthlyDistances := initializedMemberScorePeriods(members)
	for _, task := range tasks {
		endDate, ok := taskCompletedTaskEndDate(task, weekStart.Location())
		if !ok {
			continue
		}
		distance := completedDistanceForTask(task, definitions)
		if distance <= 0 {
			continue
		}
		addTaskScoreDistance(weeklyDistances, taskScoreWeekIndex(weekStart, endDate), task, members, distance)
		addTaskScoreDistance(monthlyDistances, taskScoreMonthIndex(startOfMonth(weekStart), endDate), task, members, distance)
	}

	scores := map[string]taskMemberScoreItem{}
	for _, member := range members {
		key := taskMemberScoreKey(member)
		weeklyScore := weightedTaskScore(weeklyDistances[key])
		monthlyScore := weightedTaskScore(monthlyDistances[key])
		scores[key] = taskMemberScoreItem{
			WeeklyScore:  int(math.Round(weeklyScore)),
			MonthlyScore: int(math.Round(monthlyScore)),
			CurrentScore: int(math.Round((weeklyScore + monthlyScore) / 2)),
		}
	}
	return scores
}

func currentTaskMemberScores(details map[string]taskMemberScoreItem) map[string]int {
	scores := map[string]int{}
	for memberKey, detail := range details {
		scores[memberKey] = detail.CurrentScore
	}
	return scores
}

func initializedMemberScorePeriods(members []taskMember) map[string][]int {
	values := map[string][]int{}
	for _, member := range members {
		values[taskMemberScoreKey(member)] = make([]int, taskScorePeriodCount)
	}
	return values
}

func addTaskScoreDistance(periodDistances map[string][]int, index int, task Task, members []taskMember, distance int) {
	if index < 0 || index >= taskScorePeriodCount {
		return
	}
	for _, participantKey := range taskScoreParticipantKeys(task, members) {
		distances, found := periodDistances[participantKey]
		if !found {
			continue
		}
		distances[index] += distance
	}
}

func taskScoreParticipantKeys(task Task, members []taskMember) []string {
	memberKeysByID := map[string]string{}
	memberKeysByName := map[string][]string{}
	for _, member := range members {
		key := taskMemberScoreKey(member)
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

func taskMemberScoreKey(member taskMember) string {
	if member.ID != "" {
		return member.ID
	}
	return member.Name
}

func taskScoreWeekIndex(weekStart time.Time, date time.Time) int {
	taskWeekStart := startOfISOWeek(date)
	return int(weekStart.Sub(taskWeekStart).Hours() / 24 / 7)
}

func taskScoreMonthIndex(monthStart time.Time, date time.Time) int {
	return (monthStart.Year()-date.Year())*12 + int(monthStart.Month()-date.Month())
}

func weightedTaskScore(distances []int) float64 {
	if len(distances) == 0 {
		return 0
	}
	baseline := averageDistance(distances)
	weightedTotal := 0.0
	weightTotal := 0.0
	for index, weight := range taskScoreWeights() {
		if index >= len(distances) {
			break
		}
		weightedTotal += taskScoreUnit(distances[index], baseline) * weight
		weightTotal += weight
	}
	if weightTotal == 0 {
		return 0
	}
	return weightedTotal / weightTotal * 100
}

func taskScoreWeights() [taskScorePeriodCount]float64 {
	return [taskScorePeriodCount]float64{1.5, 1.4, 1.3, 1.2, 1.1}
}

func taskScoreUnit(distance int, baseline float64) float64 {
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

func applyTaskMemberScores(members []taskMember, scores map[string]int) []taskMember {
	result := append([]taskMember(nil), members...)
	for index, member := range result {
		result[index].Score = scores[taskMemberScoreKey(member)]
	}
	return result
}

func totalTaskScore(scores map[string]int) int {
	total := 0
	for _, score := range scores {
		total += score
	}
	return total
}

func endOfMonth(date time.Time) time.Time {
	return startOfMonth(date).AddDate(0, 1, -1)
}
