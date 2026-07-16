package admind

import (
	"context"
	"log"
	"strings"
	"time"
)

func (service *Service) updateSelectedCalendarReadinessStatus(ctx context.Context, account remoteCalendarAccount, readinessStatus string) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		log.Printf("mark selected calendar readiness status: %v", errorValue)
		return
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
UPDATE calendar_remote_accounts
SET selected_calendar_readiness_status = ?, updated_at = ?
WHERE id = ?
	AND selected_calendar_id = ?
	AND selected_calendar_url = ?`,
		strings.TrimSpace(readinessStatus),
		time.Now().UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(account.ID),
		strings.TrimSpace(account.SelectedCalendarID),
		strings.TrimSpace(account.SelectedCalendarURL),
	)
	if errorValue != nil {
		log.Printf("mark selected calendar readiness status: %v", errorValue)
	}
}
