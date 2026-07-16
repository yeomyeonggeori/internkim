package admind

import (
	"context"
	"database/sql"
	"errors"
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
	for {
		database, errorValue := service.openCalendarDatabase(ctx)
		if errorValue != nil {
			logCalendarEventWindowCacheFailure("open", cacheRange.Key, errorValue)
			return sourceReader(ctx, startTime, endTime)
		}
		if !service.isCalendarEventWindowCacheEnabled() {
			database.Close()
			return sourceReader(ctx, startTime, endTime)
		}
		if events, _, found, errorValue := readCalendarEventWindowCacheEntry(ctx, database, cacheRange, time.Now()); errorValue != nil {
			database.Close()
			logCalendarEventWindowCacheFailure("read", cacheRange.Key, errorValue)
			return sourceReader(ctx, startTime, endTime)
		} else if found {
			database.Close()
			return events, nil
		}
		isBuilder, flight := service.calendarWindowBuilds.begin(cacheRange.Key)
		if isBuilder {
			return service.readCalendarEventWindowAsBuilder(ctx, database, cacheRange, startTime, endTime, sourceReader, flight)
		}
		database.Close()
		result, waitError := flight.wait(ctx)
		if waitError != nil {
			return nil, waitError
		}
		if result.shouldRetry {
			continue
		}
		if !result.hasSourceRevision {
			return sourceReader(ctx, startTime, endTime)
		}
		validationDatabase, validationOpenError := service.openCalendarDatabase(ctx)
		if validationOpenError != nil {
			logCalendarEventWindowCacheFailure("open result validation", cacheRange.Key, validationOpenError)
			return sourceReader(ctx, startTime, endTime)
		}
		isCurrent, currentError := isCalendarEventWindowCacheBuildResultCurrent(ctx, validationDatabase, result)
		validationDatabase.Close()
		if currentError != nil {
			logCalendarEventWindowCacheFailure("validate result revision", cacheRange.Key, currentError)
			return sourceReader(ctx, startTime, endTime)
		}
		if !isCurrent {
			continue
		}
		if result.errorValue != nil {
			return nil, result.errorValue
		}
		return result.events, nil
	}
}

func isCalendarEventWindowCacheBuildResultCurrent(ctx context.Context, database *sql.DB, result calendarEventWindowCacheBuildResult) (bool, error) {
	if !result.hasSourceRevision {
		return false, nil
	}
	return isCalendarEventWindowSourceRevisionCurrent(ctx, database, result.sourceRevision)
}

func isCalendarEventWindowSourceRevisionCurrent(ctx context.Context, database *sql.DB, expectedRevision int64) (bool, error) {
	currentRevision, errorValue := readCalendarEventWindowSourceRevision(ctx, database)
	if errorValue != nil {
		return false, errorValue
	}
	return currentRevision == expectedRevision, nil
}

func (service *Service) readCalendarEventWindowAsBuilder(ctx context.Context, database *sql.DB, cacheRange calendarEventWindowCacheRange, startTime time.Time, endTime time.Time, sourceReader calendarEventWindowSourceReader, flight *calendarEventWindowCacheBuildFlight) (events []calendarEvent, errorValue error) {
	defer database.Close()
	var sourceRevision int64
	var hasSourceRevision bool
	defer func() {
		service.calendarWindowBuilds.complete(cacheRange.Key, flight, calendarEventWindowCacheBuildResult{
			events:            events,
			errorValue:        errorValue,
			shouldRetry:       errors.Is(errorValue, context.Canceled) || errors.Is(errorValue, context.DeadlineExceeded),
			sourceRevision:    sourceRevision,
			hasSourceRevision: hasSourceRevision,
		})
	}()
	if cachedEvents, cachedRevision, found, cacheError := readCalendarEventWindowCacheEntry(ctx, database, cacheRange, time.Now()); cacheError != nil {
		logCalendarEventWindowCacheFailure("read", cacheRange.Key, cacheError)
		return sourceReader(ctx, startTime, endTime)
	} else if found {
		sourceRevision = cachedRevision
		hasSourceRevision = true
		return cachedEvents, nil
	}
	for range calendarEventWindowCacheMaximumBuildAttempts {
		sourceRevision = 0
		hasSourceRevision = false
		revision, revisionError := readCalendarEventWindowSourceRevision(ctx, database)
		if revisionError != nil {
			logCalendarEventWindowCacheFailure("read revision", cacheRange.Key, revisionError)
			return sourceReader(ctx, startTime, endTime)
		}
		sourceRevision = revision
		hasSourceRevision = true
		sourceEvents, sourceError := sourceReader(ctx, startTime, endTime)
		if sourceError != nil {
			return nil, sourceError
		}
		writeResult, writeError := writeCalendarEventWindowCacheEntryIfCurrent(ctx, database, cacheRange, revision, sourceEvents, time.Now())
		if writeError != nil {
			logCalendarEventWindowCacheFailure("write", cacheRange.Key, writeError)
		} else if writeResult == calendarEventWindowCacheWriteStored {
			return sourceEvents, nil
		} else if writeResult == calendarEventWindowCacheWriteRevisionChanged {
			continue
		}
		isCurrent, validationError := isCalendarEventWindowSourceRevisionCurrent(ctx, database, revision)
		if validationError != nil {
			logCalendarEventWindowCacheFailure("validate fallback revision", cacheRange.Key, validationError)
			sourceRevision = 0
			hasSourceRevision = false
			return sourceEvents, nil
		}
		if !isCurrent {
			continue
		}
		return sourceEvents, nil
	}
	sourceRevision = 0
	hasSourceRevision = false
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
