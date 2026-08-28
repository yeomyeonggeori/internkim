package admind

import (
	"context"
	"time"
)

type taskSummaryReadModel struct {
	WeeklyTasks []Task
	Metrics     taskMetrics
	Report      taskReport
}

func (service *Service) buildTaskSummaryReadModel(ctx context.Context, weekCode string, weekStart time.Time, members []taskMember) (taskSummaryReadModel, error) {
	definitions, errorValue := service.readTaskDefinitions(ctx)
	if errorValue != nil {
		return taskSummaryReadModel{}, errorValue
	}
	weeklyTasks, errorValue := service.readTasks(ctx, weekCode, members)
	if errorValue != nil {
		return taskSummaryReadModel{}, errorValue
	}
	report, errorValue := service.buildTaskReport(ctx, weekStart, members, weeklyTasks, definitions)
	if errorValue != nil {
		return taskSummaryReadModel{}, errorValue
	}
	metrics := buildTaskMetrics(weeklyTasks, definitions)
	metrics.MemberScores = map[string]int{}
	metrics.MemberScoreDetails = map[string]taskMemberScoreItem{}
	metrics.TotalScore = 0
	return taskSummaryReadModel{WeeklyTasks: weeklyTasks, Metrics: metrics, Report: report}, nil
}
