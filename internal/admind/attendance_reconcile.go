package admind

import (
	"context"
	"log"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const publishInterval = 15 * time.Minute
const publishMonthsBack = 1

func monthsToPublish(now time.Time, back int) []string {
	months := make([]string, 0, back+1)
	for step := back; step >= 0; step-- {
		months = append(months, now.AddDate(0, -step, 0).Format("2006-01"))
	}
	return months
}

func monthWindow(month string) (time.Time, time.Time, error) {
	from, errorValue := time.Parse("2006-01", month)
	if errorValue != nil {
		return time.Time{}, time.Time{}, errorValue
	}
	return from, from.AddDate(0, 1, 0), nil
}

func reconciledWorkCalendarOf(days []attendanceWorkCalendarDay) []centralplane.ReconciledWorkCalendarDay {
	reconciled := make([]centralplane.ReconciledWorkCalendarDay, 0, len(days))
	for _, day := range days {
		reconciled = append(reconciled, centralplane.ReconciledWorkCalendarDay{
			Date:        day.Date,
			WorkMode:    day.WorkMode,
			WorkingDate: day.WorkingDate,
			Holiday:     day.Holiday,
		})
	}
	return reconciled
}

func reconciledWorkPolicyOf(revision attendanceWorkPolicyRevision) centralplane.ReconciledWorkPolicy {
	breakPeriods := make([]centralplane.ReconciledWorkBreakPeriod, 0, len(revision.BreakPeriods))
	for _, period := range revision.BreakPeriods {
		breakPeriods = append(breakPeriods, centralplane.ReconciledWorkBreakPeriod{
			StartTime: period.StartTime,
			EndTime:   period.EndTime,
		})
	}
	return centralplane.ReconciledWorkPolicy{
		WorkMode:            revision.WorkMode,
		WorkingWeekdays:     append([]int(nil), revision.WorkingWeekdays...),
		DailyTargetMinutes:  revision.DailyTargetMinutes,
		WeeklyTargetMinutes: revision.WeeklyTargetMinutes,
		ReferenceStartTime:  revision.ReferenceStartTime,
		FixedStartTime:      revision.FixedStartTime,
		FixedEndTime:        revision.FixedEndTime,
		CoreTimeEnabled:     revision.CoreTimeEnabled,
		CoreStartTime:       revision.CoreStartTime,
		CoreEndTime:         revision.CoreEndTime,
		BreakPeriods:        breakPeriods,
		NightStartTime:      revision.NightStartTime,
		NightEndTime:        revision.NightEndTime,
	}
}

func (service *Service) publishWorkPolicyForMonth(ctx context.Context, month string) error {
	client := service.centralPlane()
	if client == nil {
		return nil
	}
	from, to, errorValue := monthWindow(month)
	if errorValue != nil {
		return errorValue
	}
	workCalendar, errorValue := service.attendanceWorkCalendarProjection(ctx, from, to)
	if errorValue != nil {
		return errorValue
	}
	currentPolicy, errorValue := service.currentAttendanceWorkPolicyRevision(ctx, time.Now())
	if errorValue != nil {
		return errorValue
	}
	reconciledPolicy := reconciledWorkPolicyOf(currentPolicy)

	if _, errorValue := client.ReconcileAttendance(ctx, centralplane.ReconcileWindow{
		Platform:     "mattermost",
		WorkMode:     currentPolicy.WorkMode,
		WorkPolicy:   &reconciledPolicy,
		From:         from,
		To:           to,
		WorkCalendar: reconciledWorkCalendarOf(workCalendar),
	}); errorValue != nil {
		return errorValue
	}
	return nil
}

func (service *Service) currentAttendanceWorkPolicyRevision(
	ctx context.Context,
	now time.Time,
) (attendanceWorkPolicyRevision, error) {
	policy, errorValue := service.readAttendanceWorkPolicy(ctx)
	if errorValue != nil {
		return attendanceWorkPolicyRevision{}, errorValue
	}
	location, _ := service.workspaceTimeLocation()
	revision, errorValue := attendanceWorkPolicyRevisionForDate(policy, now.In(location).Format(time.DateOnly))
	if errorValue != nil {
		return attendanceWorkPolicyRevision{}, errorValue
	}
	return revision, nil
}

func (service *Service) publishWorkPolicyRecently(ctx context.Context) {
	for _, month := range monthsToPublish(time.Now().UTC(), publishMonthsBack) {
		if errorValue := service.publishWorkPolicyForMonth(ctx, month); errorValue != nil {
			log.Printf("work policy for %s not published: %v", month, errorValue)
		}
	}
}

func (service *Service) keepWorkPolicyPublished(ctx context.Context) {
	if service.centralPlane() == nil {
		return
	}
	service.publishWorkPolicyRecently(ctx)

	ticker := time.NewTicker(publishInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.publishWorkPolicyRecently(ctx)
		}
	}
}
