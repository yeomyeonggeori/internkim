package admind

import (
	"context"
	"log"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const reconcileInterval = 15 * time.Minute
const reconcileMonthsBack = 1

func monthsToReconcile(now time.Time, back int) []string {
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

func reconciledEventsOf(events []attendanceEvent) []centralplane.ReconciledEvent {
	reconciled := make([]centralplane.ReconciledEvent, 0, len(events))
	for _, event := range events {
		externalID := strings.TrimSpace(event.MattermostUserID)
		if externalID == "" || strings.TrimSpace(event.CanceledAt) != "" {
			continue
		}
		occurredAt, errorValue := time.Parse(time.RFC3339Nano, event.OccurredAt)
		if errorValue != nil {
			continue
		}
		reconciled = append(reconciled, centralplane.ReconciledEvent{
			ExternalID: externalID,
			Kind:       event.Kind,
			Location:   event.LocationName,
			OccurredAt: occurredAt,
		})
	}
	return reconciled
}

func reconciledWorkCalendarOf(days []attendanceWorkCalendarDay) []centralplane.ReconciledWorkCalendarDay {
	reconciled := make([]centralplane.ReconciledWorkCalendarDay, 0, len(days))
	for _, day := range days {
		reconciled = append(reconciled, centralplane.ReconciledWorkCalendarDay{
			Date:        day.Date,
			WorkMode:    day.WorkMode,
			WorkingDate: day.WorkingDate,
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

func (service *Service) reconcileAttendanceMonth(ctx context.Context, month string) error {
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
	events, errorValue := service.readAttendanceEvents(ctx, month, "")
	if errorValue != nil {
		return errorValue
	}
	policy, errorValue := service.readAttendanceWorkPolicy(ctx)
	if errorValue != nil {
		return errorValue
	}
	location, _ := service.workspaceTimeLocation()
	currentPolicy, errorValue := attendanceWorkPolicyRevisionForDate(
		policy,
		time.Now().In(location).Format(time.DateOnly),
	)
	if errorValue != nil {
		return errorValue
	}
	reconciledPolicy := reconciledWorkPolicyOf(currentPolicy)

	result, errorValue := client.ReconcileAttendance(ctx, centralplane.ReconcileWindow{
		Platform:     "mattermost",
		WorkMode:     currentPolicy.WorkMode,
		WorkPolicy:   &reconciledPolicy,
		From:         from,
		To:           to,
		Events:       reconciledEventsOf(events),
		WorkCalendar: reconciledWorkCalendarOf(workCalendar),
	})
	if errorValue != nil {
		return errorValue
	}
	if result.Added > 0 || result.Removed > 0 || len(result.Refused) > 0 {
		log.Printf("attendance %s reconciled: %d added, %d removed, %d refused",
			month, result.Added, result.Removed, len(result.Refused))
	}
	return nil
}

func (service *Service) currentAttendanceWorkMode(ctx context.Context, now time.Time) (string, error) {
	policy, errorValue := service.readAttendanceWorkPolicy(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	location, _ := service.workspaceTimeLocation()
	revision, errorValue := attendanceWorkPolicyRevisionForDate(policy, now.In(location).Format(time.DateOnly))
	if errorValue != nil {
		return "", errorValue
	}
	return revision.WorkMode, nil
}

func (service *Service) reconcileAttendanceRecently(ctx context.Context) {
	for _, month := range monthsToReconcile(time.Now().UTC(), reconcileMonthsBack) {
		if errorValue := service.reconcileAttendanceMonth(ctx, month); errorValue != nil {
			log.Printf("attendance %s not reconciled: %v", month, errorValue)
		}
	}
}

func (service *Service) keepAttendanceReconciled(ctx context.Context) {
	if service.centralPlane() == nil {
		return
	}
	service.reconcileAttendanceRecently(ctx)

	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.reconcileAttendanceRecently(ctx)
		}
	}
}
