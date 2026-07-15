package admind

import (
	"context"
	"log/slog"
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
		logCalendarEventWindowCacheFailure("open", cacheRange.Key, errorValue)
		return sourceReader(ctx, startTime, endTime)
	}
	defer database.Close()
	if events, found, errorValue := readCalendarEventWindowCacheEntry(ctx, database, cacheRange, time.Now()); errorValue != nil {
		logCalendarEventWindowCacheFailure("read", cacheRange.Key, errorValue)
		return sourceReader(ctx, startTime, endTime)
	} else if found {
		return events, nil
	}
	for range calendarEventWindowCacheMaximumBuildAttempts {
		revision, errorValue := readCalendarEventWindowSourceRevision(ctx, database)
		if errorValue != nil {
			logCalendarEventWindowCacheFailure("read revision", cacheRange.Key, errorValue)
			return sourceReader(ctx, startTime, endTime)
		}
		events, errorValue := sourceReader(ctx, startTime, endTime)
		if errorValue != nil {
			return nil, errorValue
		}
		stored, errorValue := writeCalendarEventWindowCacheEntryIfCurrent(ctx, database, cacheRange, revision, events, time.Now())
		if errorValue != nil {
			logCalendarEventWindowCacheFailure("write", cacheRange.Key, errorValue)
			return events, nil
		}
		if stored {
			return events, nil
		}
	}
	return sourceReader(ctx, startTime, endTime)
}

func logCalendarEventWindowCacheFailure(operation string, cacheKey string, errorValue error) {
	slog.Warn(
		"calendar event window cache operation failed",
		"operation", operation,
		"cache_key", cacheKey,
		"error", errorValue.Error(),
	)
}
