package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCalendarEventWindowCacheSchema(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	assertCalendarEventWindowCacheColumns(t, database, "calendar_event_window_source_state", "id,revision")
	assertCalendarEventWindowCacheColumns(t, database, "calendar_event_window_cache_entries", "cache_key,start_at,end_at,source_revision,schema_version,payload_json,cached_at,last_used_at")
}

func TestCalendarEventWindowCacheRangeNormalizesUTC(t *testing.T) {
	seoul := time.FixedZone("Asia/Seoul", 9*60*60)
	seoulStart := time.Date(2026, time.July, 15, 9, 0, 0, 0, seoul)
	seoulEnd := seoulStart.Add(24 * time.Hour)
	utcStart := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	utcEnd := utcStart.Add(24 * time.Hour)
	seoulRange, seoulValid := calendarEventWindowCacheRangeFor(seoulStart, seoulEnd)
	utcRange, utcValid := calendarEventWindowCacheRangeFor(utcStart, utcEnd)
	if !seoulValid || !utcValid {
		t.Fatalf("cache range validity = %v, %v", seoulValid, utcValid)
	}
	if seoulRange != utcRange {
		t.Fatalf("Seoul range = %+v, UTC range = %+v", seoulRange, utcRange)
	}
	if utcRange.Key != "2026-07-15T00:00:00.000000000Z/2026-07-16T00:00:00.000000000Z" {
		t.Fatalf("cache key = %q", utcRange.Key)
	}
}

func TestCalendarEventWindowCachePurgesEntriesForNewService(t *testing.T) {
	service := newCalendarTestService(t)
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	if _, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime); errorValue != nil {
		t.Fatal(errorValue)
	}
	restartedService := NewService(service.Configuration)
	database, errorValue := restartedService.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var cacheEntryCount int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries").Scan(&cacheEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheEntryCount != 0 {
		t.Fatalf("cache entries after restart = %d, want 0", cacheEntryCount)
	}
}

func TestCalendarEventWindowCacheSkipsWideRange(t *testing.T) {
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	if _, cacheable := calendarEventWindowCacheRangeFor(startTime, startTime.Add(63*24*time.Hour)); cacheable {
		t.Fatal("63 day range is cacheable")
	}
}

func TestCalendarEventWindowCacheReusesStoredProjection(t *testing.T) {
	service := newCalendarTestService(t)
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	event := calendarEvent{
		ID:                "cached-event",
		UID:               "cached-event@example.test",
		Title:             "Cached event",
		StartISO:          startTime.Add(time.Hour).Format(time.RFC3339),
		EndISO:            startTime.Add(2 * time.Hour).Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: calendarDefaultReminderLeadHours,
		CreatedByEmail:    "staff@example.com",
		Participants: []calendarParticipant{
			{PersonID: stableFlowID("staff@example.com"), Name: "Staff", Email: "staff@example.com"},
		},
	}
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	firstEvents, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(firstEvents) != 1 || len(firstEvents[0].Participants) != 1 {
		t.Fatalf("first cached events = %+v", firstEvents)
	}
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(), "UPDATE calendar_events SET deleted_at = ? WHERE id = ?", time.Now().UTC().Format(time.RFC3339Nano), event.ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	secondEvents, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(secondEvents) != 1 || secondEvents[0].ID != event.ID {
		t.Fatalf("second cached events = %+v", secondEvents)
	}
}

func TestCalendarEventWindowCacheRebuildsInvalidEntries(t *testing.T) {
	testCases := []struct {
		name          string
		schemaVersion int
		payloadJSON   string
	}{
		{name: "corrupt payload", schemaVersion: calendarEventWindowCacheSchemaVersion, payloadJSON: "{"},
		{name: "schema mismatch", schemaVersion: calendarEventWindowCacheSchemaVersion + 1, payloadJSON: `{"version":1,"events":[]}`},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newCalendarTestService(t)
			startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
			endTime := startTime.Add(24 * time.Hour)
			event := calendarEvent{
				ID:                "rebuilt-event",
				UID:               "rebuilt-event@example.test",
				Title:             "Rebuilt event",
				StartISO:          startTime.Add(time.Hour).Format(time.RFC3339),
				EndISO:            startTime.Add(2 * time.Hour).Format(time.RFC3339),
				TimeZone:          "UTC",
				Color:             "#2563eb",
				ReminderLeadHours: calendarDefaultReminderLeadHours,
				CreatedByEmail:    "staff@example.com",
			}
			if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
				t.Fatal(errorValue)
			}
			if _, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime); errorValue != nil {
				t.Fatal(errorValue)
			}
			cacheRange, _ := calendarEventWindowCacheRangeFor(startTime, endTime)
			database, errorValue := service.openCalendarDatabase(context.Background())
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if _, errorValue := database.ExecContext(context.Background(), `
				UPDATE calendar_event_window_cache_entries
				SET schema_version = ?, payload_json = ?
				WHERE cache_key = ?`, testCase.schemaVersion, testCase.payloadJSON, cacheRange.Key); errorValue != nil {
				database.Close()
				t.Fatal(errorValue)
			}
			if errorValue := database.Close(); errorValue != nil {
				t.Fatal(errorValue)
			}
			events, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if len(events) != 1 || events[0].ID != event.ID {
				t.Fatalf("rebuilt events = %+v", events)
			}
			database, errorValue = service.openCalendarDatabase(context.Background())
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			defer database.Close()
			var rebuiltSchemaVersion int
			var rebuiltPayloadJSON []byte
			if errorValue := database.QueryRowContext(context.Background(), `
				SELECT schema_version, payload_json
				FROM calendar_event_window_cache_entries
				WHERE cache_key = ?`, cacheRange.Key).Scan(&rebuiltSchemaVersion, &rebuiltPayloadJSON); errorValue != nil {
				t.Fatal(errorValue)
			}
			var rebuiltPayload calendarEventWindowCachePayload
			if errorValue := json.Unmarshal(rebuiltPayloadJSON, &rebuiltPayload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if rebuiltSchemaVersion != calendarEventWindowCacheSchemaVersion || len(rebuiltPayload.Events) != 1 || rebuiltPayload.Events[0].ID != event.ID {
				t.Fatalf("rebuilt cache schema = %d payload = %+v", rebuiltSchemaVersion, rebuiltPayload)
			}
		})
	}
}

func TestCalendarEventWindowCacheKeepsMostRecentlyUsedEntries(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	revision, errorValue := readCalendarEventWindowSourceRevision(context.Background(), database)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	baseTime := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	var oldestRange calendarEventWindowCacheRange
	var secondOldestRange calendarEventWindowCacheRange
	for index := range calendarEventWindowCacheMaximumEntries {
		startTime := baseTime.Add(time.Duration(index) * 24 * time.Hour)
		cacheRange, _ := calendarEventWindowCacheRangeFor(startTime, startTime.Add(24*time.Hour))
		if index == 0 {
			oldestRange = cacheRange
		}
		if index == 1 {
			secondOldestRange = cacheRange
		}
		writeResult, errorValue := writeCalendarEventWindowCacheEntryIfCurrent(context.Background(), database, cacheRange, revision, nil, baseTime.Add(time.Duration(index)*time.Second))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if writeResult != calendarEventWindowCacheWriteStored {
			t.Fatalf("cache entry %d was not stored", index)
		}
	}
	if _, found, errorValue := readCalendarEventWindowCacheEntry(context.Background(), database, oldestRange, baseTime.Add(time.Hour)); errorValue != nil {
		t.Fatal(errorValue)
	} else if !found {
		t.Fatal("oldest cache entry was not found")
	}
	newStartTime := baseTime.Add(time.Duration(calendarEventWindowCacheMaximumEntries) * 24 * time.Hour)
	newRange, _ := calendarEventWindowCacheRangeFor(newStartTime, newStartTime.Add(24*time.Hour))
	if writeResult, errorValue := writeCalendarEventWindowCacheEntryIfCurrent(context.Background(), database, newRange, revision, nil, baseTime.Add(time.Duration(calendarEventWindowCacheMaximumEntries)*time.Second)); errorValue != nil {
		t.Fatal(errorValue)
	} else if writeResult != calendarEventWindowCacheWriteStored {
		t.Fatal("new cache entry was not stored")
	}
	var cacheEntryCount int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries").Scan(&cacheEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	var oldestEntryCount int
	var secondOldestEntryCount int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries WHERE cache_key = ?", oldestRange.Key).Scan(&oldestEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries WHERE cache_key = ?", secondOldestRange.Key).Scan(&secondOldestEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheEntryCount != calendarEventWindowCacheMaximumEntries || oldestEntryCount != 1 || secondOldestEntryCount != 0 {
		t.Fatalf("cache entries = %d oldest entries = %d second oldest entries = %d", cacheEntryCount, oldestEntryCount, secondOldestEntryCount)
	}
}

func TestCalendarEventWindowCacheSkipsOversizedPayload(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	revision, errorValue := readCalendarEventWindowSourceRevision(context.Background(), database)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	cacheRange, _ := calendarEventWindowCacheRangeFor(startTime, startTime.Add(24*time.Hour))
	events := []calendarEvent{{ID: "oversized", Description: strings.Repeat("x", (4<<20)+1)}}
	writeResult, errorValue := writeCalendarEventWindowCacheEntryIfCurrent(context.Background(), database, cacheRange, revision, events, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if writeResult != calendarEventWindowCacheWriteSkipped {
		t.Fatalf("oversized payload write result = %d", writeResult)
	}
}

func TestCalendarEventWindowCacheThrottlesLRUTouch(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	revision, errorValue := readCalendarEventWindowSourceRevision(context.Background(), database)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	baseTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	cacheRange, _ := calendarEventWindowCacheRangeFor(baseTime, baseTime.Add(24*time.Hour))
	if writeResult, errorValue := writeCalendarEventWindowCacheEntryIfCurrent(context.Background(), database, cacheRange, revision, nil, baseTime); errorValue != nil {
		t.Fatal(errorValue)
	} else if writeResult != calendarEventWindowCacheWriteStored {
		t.Fatal("cache entry was not stored")
	}
	if _, found, errorValue := readCalendarEventWindowCacheEntry(context.Background(), database, cacheRange, baseTime.Add(30*time.Second)); errorValue != nil {
		t.Fatal(errorValue)
	} else if !found {
		t.Fatal("cache entry was not found")
	}
	var lastUsedAt string
	if errorValue := database.QueryRowContext(context.Background(), "SELECT last_used_at FROM calendar_event_window_cache_entries WHERE cache_key = ?", cacheRange.Key).Scan(&lastUsedAt); errorValue != nil {
		t.Fatal(errorValue)
	}
	if lastUsedAt != formatCalendarEventWindowCacheTimestamp(baseTime) {
		t.Fatalf("last used at after early hit = %q", lastUsedAt)
	}
	lateTouchTime := baseTime.Add(2 * time.Minute)
	if _, found, errorValue := readCalendarEventWindowCacheEntry(context.Background(), database, cacheRange, lateTouchTime); errorValue != nil {
		t.Fatal(errorValue)
	} else if !found {
		t.Fatal("cache entry was not found")
	}
	if errorValue := database.QueryRowContext(context.Background(), "SELECT last_used_at FROM calendar_event_window_cache_entries WHERE cache_key = ?", cacheRange.Key).Scan(&lastUsedAt); errorValue != nil {
		t.Fatal(errorValue)
	}
	if lastUsedAt != formatCalendarEventWindowCacheTimestamp(lateTouchTime) {
		t.Fatalf("last used at after late hit = %q", lastUsedAt)
	}
}

func TestCalendarEventWindowCacheRollsBackFailedCleanup(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	revision, errorValue := readCalendarEventWindowSourceRevision(context.Background(), database)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	baseTime := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	for index := range calendarEventWindowCacheMaximumEntries {
		startTime := baseTime.Add(time.Duration(index) * 24 * time.Hour)
		cacheRange, _ := calendarEventWindowCacheRangeFor(startTime, startTime.Add(24*time.Hour))
		if writeResult, errorValue := writeCalendarEventWindowCacheEntryIfCurrent(context.Background(), database, cacheRange, revision, nil, baseTime.Add(time.Duration(index)*time.Second)); errorValue != nil {
			t.Fatal(errorValue)
		} else if writeResult != calendarEventWindowCacheWriteStored {
			t.Fatalf("cache entry %d was not stored", index)
		}
	}
	if _, errorValue := database.ExecContext(context.Background(), `
		CREATE TRIGGER fail_calendar_event_window_cache_cleanup
		BEFORE DELETE ON calendar_event_window_cache_entries
		BEGIN
			SELECT RAISE(FAIL, 'cache cleanup failed');
		END`); errorValue != nil {
		t.Fatal(errorValue)
	}
	newStartTime := baseTime.Add(time.Duration(calendarEventWindowCacheMaximumEntries) * 24 * time.Hour)
	newRange, _ := calendarEventWindowCacheRangeFor(newStartTime, newStartTime.Add(24*time.Hour))
	if _, errorValue := writeCalendarEventWindowCacheEntryIfCurrent(context.Background(), database, newRange, revision, nil, baseTime.Add(time.Hour)); errorValue == nil {
		t.Fatal("cache cleanup succeeded")
	}
	var cacheEntryCount int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries").Scan(&cacheEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheEntryCount != calendarEventWindowCacheMaximumEntries {
		t.Fatalf("cache entries after failed cleanup = %d, want %d", cacheEntryCount, calendarEventWindowCacheMaximumEntries)
	}
}

func assertCalendarEventWindowCacheColumns(testContext testing.TB, database *sql.DB, tableName string, expectedColumns string) {
	testContext.Helper()
	rows, errorValue := database.QueryContext(context.Background(), "PRAGMA table_info("+tableName+")")
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	defer rows.Close()
	columns := ""
	for rows.Next() {
		var identifier int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if errorValue := rows.Scan(&identifier, &name, &columnType, &notNull, &defaultValue, &primaryKey); errorValue != nil {
			testContext.Fatal(errorValue)
		}
		if columns != "" {
			columns += ","
		}
		columns += name
	}
	if errorValue := rows.Err(); errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if columns != expectedColumns {
		testContext.Fatalf("%s columns = %q, want %q", tableName, columns, expectedColumns)
	}
}
