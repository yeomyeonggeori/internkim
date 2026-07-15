package admind

import (
	"context"
	"time"
)

const calendarEventWindowCacheMaximumBuildAttempts = 3

type calendarEventWindowSourceReader func(context.Context, time.Time, time.Time) ([]calendarEvent, error)

func (service *Service) readCalendarEventWindow(ctx context.Context, startTime time.Time, endTime time.Time) ([]calendarEvent, error) {
	return service.readCalendarEventWindowWithSourceReader(ctx, startTime, endTime, service.readCalendarEvents)
}

func (service *Service) readCalendarEventWindowWithSourceReader(ctx context.Context, startTime time.Time, endTime time.Time, sourceReader calendarEventWindowSourceReader) ([]calendarEvent, error) {
	cacheRange, cacheable := calendarEventWindowCacheRangeFor(startTime, endTime)
	if !cacheable {
		return sourceReader(ctx, startTime, endTime)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	if events, found, errorValue := readCalendarEventWindowCacheEntry(ctx, database, cacheRange, time.Now()); errorValue != nil {
		return nil, errorValue
	} else if found {
		return events, nil
	}
	for range calendarEventWindowCacheMaximumBuildAttempts {
		revision, errorValue := readCalendarEventWindowSourceRevision(ctx, database)
		if errorValue != nil {
			return nil, errorValue
		}
		events, errorValue := sourceReader(ctx, startTime, endTime)
		if errorValue != nil {
			return nil, errorValue
		}
		stored, errorValue := writeCalendarEventWindowCacheEntryIfCurrent(ctx, database, cacheRange, revision, events, time.Now())
		if errorValue != nil {
			return nil, errorValue
		}
		if stored {
			return events, nil
		}
	}
	return sourceReader(ctx, startTime, endTime)
}
