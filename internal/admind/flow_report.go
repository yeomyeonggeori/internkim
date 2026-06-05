package admind

import (
	"context"
	"strconv"
	"strings"
	"time"
)

func (service *Service) buildFlowReport(ctx context.Context, weekStart time.Time, members []flowMember, tasks []flowTask, definitions flowDefinitions) (flowReport, error) {
	previousWeekTasks, errorValue := service.readFlowTasks(ctx, weekCodeForDate(weekStart.AddDate(0, 0, -7)), members)
	if errorValue != nil {
		return flowReport{}, errorValue
	}

	currentMonthStart := startOfMonth(weekStart)
	currentMonthEnd := currentMonthStart.AddDate(0, 1, -1)
	previousMonthStart := currentMonthStart.AddDate(0, -1, 0)
	previousMonthEnd := currentMonthStart.AddDate(0, 0, -1)

	currentMonthTasks, errorValue := service.readFlowTasksBetweenDates(ctx, currentMonthStart.Format("2006-01-02"), currentMonthEnd.Format("2006-01-02"), members)
	if errorValue != nil {
		return flowReport{}, errorValue
	}
	previousMonthTasks, errorValue := service.readFlowTasksBetweenDates(ctx, previousMonthStart.Format("2006-01-02"), previousMonthEnd.Format("2006-01-02"), members)
	if errorValue != nil {
		return flowReport{}, errorValue
	}

	return flowReport{
		WeeklyDistanceTrend:  buildWeeklyDistanceTrend(tasks, previousWeekTasks, definitions, weekStart),
		MonthlyDistanceTrend: buildMonthlyDistanceTrend(currentMonthTasks, previousMonthTasks, definitions, currentMonthStart, previousMonthStart),
	}, nil
}

func buildWeeklyDistanceTrend(currentTasks []flowTask, previousTasks []flowTask, definitions flowDefinitions, weekStart time.Time) flowDistanceTrend {
	currentValues := cumulativeDailyTeamDistances(currentTasks, definitions, weekStart, 7)
	previousValues := cumulativeDailyTeamDistances(previousTasks, definitions, weekStart.AddDate(0, 0, -7), 7)
	return flowDistanceTrend{
		Labels:         []string{"월", "화", "수", "목", "금", "토", "일"},
		CurrentLabel:   "이번 주",
		PreviousLabel:  "지난 주",
		CurrentValues:  currentValues,
		PreviousValues: previousValues,
		CurrentTotal:   lastInt(currentValues),
		PreviousTotal:  lastInt(previousValues),
		Unit:           "km",
	}
}

func buildMonthlyDistanceTrend(currentTasks []flowTask, previousTasks []flowTask, definitions flowDefinitions, currentStart time.Time, previousStart time.Time) flowDistanceTrend {
	dayCount := daysInMonth(currentStart)
	currentValues := cumulativeDailyTeamDistances(currentTasks, definitions, currentStart, dayCount)
	previousValues := cumulativeDailyTeamDistances(previousTasks, definitions, previousStart, dayCount)
	return flowDistanceTrend{
		Labels:         dayNumberLabels(dayCount),
		CurrentLabel:   "이번 달",
		PreviousLabel:  "지난 달",
		CurrentValues:  currentValues,
		PreviousValues: previousValues,
		CurrentTotal:   lastInt(currentValues),
		PreviousTotal:  lastInt(previousValues),
		Unit:           "km",
	}
}

func cumulativeDailyTeamDistances(tasks []flowTask, definitions flowDefinitions, startDate time.Time, dayCount int) []int {
	dailyDistances := make([]int, dayCount)
	for _, task := range tasks {
		scoreDate, ok := flowTaskScoreDate(task, startDate.Location())
		if !ok {
			continue
		}
		dayIndex := int(scoreDate.Sub(startDate).Hours() / 24)
		if dayIndex < 0 || dayIndex >= dayCount {
			continue
		}
		dailyDistances[dayIndex] += progressDistanceForTask(task, definitions) * flowTaskParticipantCount(task)
	}

	values := make([]int, 0, dayCount)
	total := 0
	for _, distance := range dailyDistances {
		total += distance
		values = append(values, total)
	}
	return values
}

func daysInMonth(date time.Time) int {
	return date.AddDate(0, 1, -date.Day()).Day()
}

func dayNumberLabels(dayCount int) []string {
	labels := make([]string, 0, dayCount)
	for day := 1; day <= dayCount; day++ {
		labels = append(labels, strconv.Itoa(day))
	}
	return labels
}

func flowTaskParticipantCount(task flowTask) int {
	if len(task.ParticipantIDs) > 0 {
		return len(task.ParticipantIDs)
	}
	if len(task.ParticipantNames) > 0 {
		return len(task.ParticipantNames)
	}
	return 1
}

func flowTaskScoreDate(task flowTask, location *time.Location) (time.Time, bool) {
	value := strings.TrimSpace(task.StartDate)
	if task.Status == "완료" && strings.TrimSpace(task.EndDate) != "" {
		value = strings.TrimSpace(task.EndDate)
	}
	if value == "" {
		return time.Time{}, false
	}
	parsedDate, errorValue := time.ParseInLocation("2006-01-02", value, location)
	return parsedDate, errorValue == nil
}

func lastInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[len(values)-1]
}

func startOfMonth(date time.Time) time.Time {
	year, month, _ := date.Date()
	return time.Date(year, month, 1, 0, 0, 0, 0, date.Location())
}
