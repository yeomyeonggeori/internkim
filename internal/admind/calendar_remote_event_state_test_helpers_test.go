package admind

import (
	"context"
	"time"
)

func (service *Service) upsertCalendarRemoteEventState(ctx context.Context, state calendarRemoteEventState) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	return upsertCalendarRemoteEventStateWithRunner(ctx, database, state, time.Now().UTC().Format(time.RFC3339Nano))
}
