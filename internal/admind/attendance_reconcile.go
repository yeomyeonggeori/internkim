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

func (service *Service) reconcileAttendanceMonth(ctx context.Context, month string) error {
	client := service.centralPlane()
	if client == nil {
		return nil
	}
	from, to, errorValue := monthWindow(month)
	if errorValue != nil {
		return errorValue
	}
	events, errorValue := service.readAttendanceEvents(ctx, month, "")
	if errorValue != nil {
		return errorValue
	}

	result, errorValue := client.ReconcileAttendance(ctx, centralplane.ReconcileWindow{
		Platform: "mattermost",
		From:     from,
		To:       to,
		Events:   reconciledEventsOf(events),
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
