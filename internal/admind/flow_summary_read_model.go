package admind

import (
	"context"
	"time"
)

type flowSummaryReadModel struct {
	WeeklyTasks []flowTask
	Metrics     flowMetrics
	Report      flowReport
}

func (service *Service) buildFlowSummaryReadModel(ctx context.Context, weekCode string, weekStart time.Time, members []flowMember) (flowSummaryReadModel, error) {
	definitions, errorValue := service.readFlowDefinitions(ctx)
	if errorValue != nil {
		return flowSummaryReadModel{}, errorValue
	}
	weeklyTasks, errorValue := service.readFlowTasks(ctx, weekCode, members)
	if errorValue != nil {
		return flowSummaryReadModel{}, errorValue
	}
	report, errorValue := service.buildFlowReport(ctx, weekStart, members, weeklyTasks, definitions)
	if errorValue != nil {
		return flowSummaryReadModel{}, errorValue
	}
	metrics := buildFlowMetrics(weeklyTasks, definitions)
	metrics.MemberScores = map[string]int{}
	metrics.MemberScoreDetails = map[string]flowMemberScoreItem{}
	metrics.TotalScore = 0
	return flowSummaryReadModel{WeeklyTasks: weeklyTasks, Metrics: metrics, Report: report}, nil
}
