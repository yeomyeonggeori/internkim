package admind

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCalendarEventWindowCacheFollowerRetriesErrorAfterRevisionChange(t *testing.T) {
	service := newCalendarTestService(t)
	startTime := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	cacheRange, cacheable := calendarEventWindowCacheRangeFor(startTime, endTime)
	if !cacheable {
		t.Fatal("test range is not cacheable")
	}
	sentinelError := errors.New("source failed before revision change")
	freshEvent := calendarEvent{ID: "fresh-after-error-revision-change"}
	var sourceReadCount atomic.Int32
	firstSourceReadStarted := make(chan struct{})
	releaseFirstSourceRead := make(chan struct{})
	sourceReader := func(ctx context.Context, rangeStart time.Time, rangeEnd time.Time) ([]calendarEvent, error) {
		if sourceReadCount.Add(1) != 1 {
			return []calendarEvent{freshEvent}, nil
		}
		close(firstSourceReadStarted)
		select {
		case <-releaseFirstSourceRead:
			return nil, sentinelError
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	testContext, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	type readResult struct {
		events []calendarEvent
		error  error
	}
	builderResult := make(chan readResult, 1)
	go func() {
		events, errorValue := service.readCalendarEventWindowWithSourceReader(testContext, startTime, endTime, sourceReader)
		builderResult <- readResult{events: events, error: errorValue}
	}()
	select {
	case <-firstSourceReadStarted:
	case <-testContext.Done():
		t.Fatal(testContext.Err())
	}
	database, errorValue := service.openCalendarDatabase(testContext)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, updateError := database.ExecContext(testContext, "UPDATE calendar_event_window_source_state SET revision = revision + 1 WHERE id = 1")
	closeError := database.Close()
	if updateError != nil {
		t.Fatal(updateError)
	}
	if closeError != nil {
		t.Fatal(closeError)
	}
	followerResult := make(chan readResult, 1)
	go func() {
		events, errorValue := service.readCalendarEventWindowWithSourceReader(testContext, startTime, endTime, sourceReader)
		followerResult <- readResult{events: events, error: errorValue}
	}()
	waitForCalendarEventWindowCacheFollowers(t, testContext, service, cacheRange.Key, 1)
	close(releaseFirstSourceRead)
	select {
	case result := <-builderResult:
		if !errors.Is(result.error, sentinelError) {
			t.Fatalf("builder error = %v, want sentinel error", result.error)
		}
	case <-testContext.Done():
		t.Fatal(testContext.Err())
	}
	select {
	case result := <-followerResult:
		if result.error != nil {
			t.Fatalf("follower error = %v, want nil", result.error)
		}
		if len(result.events) != 1 {
			t.Fatalf("follower event count = %d, want 1", len(result.events))
		}
		if result.events[0].ID != freshEvent.ID {
			t.Fatalf("follower event ID = %q, want %q", result.events[0].ID, freshEvent.ID)
		}
	case <-testContext.Done():
		t.Fatal(testContext.Err())
	}
	if sourceReadCount.Load() != 2 {
		t.Fatalf("source read count = %d, want 2", sourceReadCount.Load())
	}
}

func TestCalendarEventWindowCacheBuilderAndFollowerReceiveRebuiltResultAfterRevisionChange(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	startTime := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	cacheRange, cacheable := calendarEventWindowCacheRangeFor(startTime, endTime)
	if !cacheable {
		t.Fatal("test range is not cacheable")
	}
	oldEvents := []calendarEvent{{
		ID:          "old-result",
		Description: strings.Repeat("x", calendarEventWindowCacheMaximumPayloadBytes),
	}}
	freshEvent := calendarEvent{ID: "fresh-result"}
	var sourceReadCount atomic.Int32
	firstSourceReadStarted := make(chan struct{})
	releaseFirstSourceRead := make(chan struct{})
	sourceReader := func(ctx context.Context, rangeStart time.Time, rangeEnd time.Time) ([]calendarEvent, error) {
		if sourceReadCount.Add(1) == 1 {
			close(firstSourceReadStarted)
			select {
			case <-releaseFirstSourceRead:
				return oldEvents, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return []calendarEvent{freshEvent}, nil
	}
	testContext, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	type readResult struct {
		events []calendarEvent
		error  error
	}
	builderResult := make(chan readResult, 1)
	go func() {
		events, errorValue := service.readCalendarEventWindowWithSourceReader(testContext, startTime, endTime, sourceReader)
		builderResult <- readResult{events: events, error: errorValue}
	}()
	select {
	case <-firstSourceReadStarted:
	case <-testContext.Done():
		t.Fatal(testContext.Err())
	}
	database, errorValue = service.openCalendarDatabase(testContext)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(testContext, "UPDATE calendar_event_window_source_state SET revision = revision + 1 WHERE id = 1"); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	followerResult := make(chan readResult, 1)
	go func() {
		events, errorValue := service.readCalendarEventWindowWithSourceReader(testContext, startTime, endTime, sourceReader)
		followerResult <- readResult{events: events, error: errorValue}
	}()
	waitForCalendarEventWindowCacheFollowers(t, testContext, service, cacheRange.Key, 1)
	close(releaseFirstSourceRead)
	select {
	case result := <-builderResult:
		if result.error != nil {
			t.Fatal(result.error)
		}
		if len(result.events) != 1 {
			t.Fatalf("builder event count = %d, want 1", len(result.events))
		}
		if result.events[0].ID != freshEvent.ID {
			t.Fatalf("builder event ID = %q, want %q", result.events[0].ID, freshEvent.ID)
		}
	case <-testContext.Done():
		t.Fatal(testContext.Err())
	}
	select {
	case result := <-followerResult:
		if result.error != nil {
			t.Fatal(result.error)
		}
		if len(result.events) != 1 {
			t.Fatalf("follower event count = %d, want 1", len(result.events))
		}
		if result.events[0].ID != freshEvent.ID {
			t.Fatalf("follower event ID = %q, want %q", result.events[0].ID, freshEvent.ID)
		}
	case <-testContext.Done():
		t.Fatal(testContext.Err())
	}
	if sourceReadCount.Load() != 2 {
		t.Fatalf("source read count = %d, want 2", sourceReadCount.Load())
	}
}

func TestCalendarEventWindowCacheFollowerRebuildsAfterBuilderCancellation(t *testing.T) {
	service := newCalendarTestService(t)
	testContext, cancelTest := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelTest()
	database, errorValue := service.openCalendarDatabase(testContext)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	startTime := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	cacheRange, cacheable := calendarEventWindowCacheRangeFor(startTime, endTime)
	if !cacheable {
		t.Fatal("test range is not cacheable")
	}
	expectedEvent := calendarEvent{ID: "recovered", Title: "Recovered"}
	var sourceReadCount atomic.Int32
	sourceReadStarted := make(chan int32, 2)
	sourceReader := func(ctx context.Context, rangeStart time.Time, rangeEnd time.Time) ([]calendarEvent, error) {
		readCount := sourceReadCount.Add(1)
		select {
		case sourceReadStarted <- readCount:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if readCount == 1 {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []calendarEvent{expectedEvent}, nil
	}
	type readResult struct {
		events []calendarEvent
		error  error
	}
	builderContext, cancelBuilder := context.WithCancel(testContext)
	builderResult := make(chan readResult, 1)
	go func() {
		events, errorValue := service.readCalendarEventWindowWithSourceReader(builderContext, startTime, endTime, sourceReader)
		builderResult <- readResult{events: events, error: errorValue}
	}()
	select {
	case readCount := <-sourceReadStarted:
		if readCount != 1 {
			t.Fatalf("first source read count = %d, want 1", readCount)
		}
	case <-testContext.Done():
		t.Fatal(testContext.Err())
	}
	followerResult := make(chan readResult, 1)
	go func() {
		events, errorValue := service.readCalendarEventWindowWithSourceReader(testContext, startTime, endTime, sourceReader)
		followerResult <- readResult{events: events, error: errorValue}
	}()
	waitForCalendarEventWindowCacheFollowers(t, testContext, service, cacheRange.Key, 1)
	cancelBuilder()
	select {
	case result := <-builderResult:
		if !errors.Is(result.error, context.Canceled) {
			t.Fatalf("builder error = %v, want context canceled", result.error)
		}
	case <-testContext.Done():
		t.Fatal(testContext.Err())
	}
	select {
	case readCount := <-sourceReadStarted:
		if readCount != 2 {
			t.Fatalf("second source read count = %d, want 2", readCount)
		}
	case <-testContext.Done():
		t.Fatal(testContext.Err())
	}
	select {
	case result := <-followerResult:
		if result.error != nil {
			t.Fatal(result.error)
		}
		if len(result.events) != 1 {
			t.Fatalf("follower event count = %d, want 1", len(result.events))
		}
		if result.events[0].ID != expectedEvent.ID {
			t.Fatalf("follower event ID = %q, want %q", result.events[0].ID, expectedEvent.ID)
		}
	case <-testContext.Done():
		t.Fatal(testContext.Err())
	}
}
