package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

const (
	calendarHolidaySourceAPI     = "holiday_api"
	calendarHolidayProviderNager = "nager"
	calendarHolidayColor         = "#dc2626"
)

type calendarHoliday struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Date        string `json:"date"`
	Source      string `json:"source"`
	CountryCode string `json:"countryCode,omitempty"`
	ReadOnly    bool   `json:"readOnly"`
	Color       string `json:"color"`
}

type calendarHolidaysResponse struct {
	Holidays []calendarHoliday `json:"holidays"`
	Source   string            `json:"source"`
}

type storedCalendarHoliday struct {
	Source      string
	SourceKey   string
	ExternalID  string
	CountryCode string
	Title       string
	Date        string
}

type calendarHolidaySourceState struct {
	Provider     string
	SourceKey    string
	LastSyncedAt string
	LastError    string
}

type calendarHolidaySnapshot struct {
	SourceKey string
	Holidays  []storedCalendarHoliday
}

func (service *Service) readCalendarHolidays(
	ctx context.Context,
	source string,
	countryCode string,
	locale string,
	startDate string,
	endDate string,
) ([]calendarHoliday, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	query := `
SELECT source_key, external_id, country_code, title, holiday_date
FROM calendar_holidays
WHERE source = ? AND holiday_date >= ? AND holiday_date < ? AND country_code = ? AND source_key LIKE ?`
	arguments := []any{source, startDate, endDate, countryCode, "%:" + locale}
	query += " ORDER BY holiday_date, title, source_key, external_id"
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	holidays := make([]calendarHoliday, 0)
	for rows.Next() {
		var sourceKey string
		var externalID string
		var holiday calendarHoliday
		if errorValue := rows.Scan(&sourceKey, &externalID, &holiday.CountryCode, &holiday.Title, &holiday.Date); errorValue != nil {
			return nil, errorValue
		}
		holiday.ID = calendarHolidayID(source, sourceKey, externalID)
		holiday.Source = source
		holiday.ReadOnly = true
		holiday.Color = calendarHolidayColor
		holidays = append(holidays, holiday)
	}
	return holidays, rows.Err()
}

func calendarHolidayID(source string, sourceKey string, externalID string) string {
	return "holiday:" + strings.TrimSpace(source) + ":" + strings.TrimSpace(sourceKey) + ":" + strings.TrimSpace(externalID)
}

func (service *Service) replaceCalendarHolidaySnapshot(
	ctx context.Context,
	source string,
	sourceKey string,
	holidays []storedCalendarHoliday,
	fetchedAt time.Time,
) error {
	return service.replaceCalendarHolidaySnapshots(
		ctx,
		source,
		[]calendarHolidaySnapshot{{SourceKey: sourceKey, Holidays: holidays}},
		nil,
		fetchedAt,
	)
}

func (service *Service) replaceCalendarHolidaySnapshots(
	ctx context.Context,
	source string,
	snapshots []calendarHolidaySnapshot,
	states []calendarHolidaySourceState,
	fetchedAt time.Time,
) error {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	defer transaction.Rollback()
	for _, snapshot := range snapshots {
		if _, errorValue := transaction.ExecContext(ctx,
			"DELETE FROM calendar_holidays WHERE source = ? AND source_key = ?",
			source,
			snapshot.SourceKey,
		); errorValue != nil {
			return errorValue
		}
		for _, holiday := range snapshot.Holidays {
			if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO calendar_holidays (
	source, source_key, external_id, country_code, title, holiday_date, fetched_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				source,
				snapshot.SourceKey,
				holiday.ExternalID,
				holiday.CountryCode,
				holiday.Title,
				holiday.Date,
				fetchedAt.UTC().Format(time.RFC3339),
			); errorValue != nil {
				return errorValue
			}
		}
	}
	for _, state := range states {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO calendar_holiday_sources (
	provider, source_key, last_synced_at, last_error
) VALUES (?, ?, ?, ?)
ON CONFLICT(provider, source_key) DO UPDATE SET
	last_synced_at = excluded.last_synced_at,
	last_error = excluded.last_error`,
			state.Provider,
			state.SourceKey,
			state.LastSyncedAt,
			state.LastError,
		); errorValue != nil {
			return errorValue
		}
	}
	return transaction.Commit()
}

func (service *Service) readCalendarHolidaySource(
	ctx context.Context,
	provider string,
	sourceKey string,
) (calendarHolidaySourceState, bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarHolidaySourceState{}, false, errorValue
	}
	defer database.Close()
	return readCalendarHolidaySourceWithRunner(ctx, database, provider, sourceKey)
}

func (service *Service) upsertCalendarHolidaySourceState(
	ctx context.Context,
	source calendarHolidaySourceState,
) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO calendar_holiday_sources (
	provider, source_key, last_synced_at, last_error
) VALUES (?, ?, ?, ?)
ON CONFLICT(provider, source_key) DO UPDATE SET
	last_synced_at = excluded.last_synced_at,
	last_error = excluded.last_error`,
		source.Provider,
		source.SourceKey,
		source.LastSyncedAt,
		source.LastError,
	)
	return errorValue
}

func calendarHolidaySourceIsFresh(timestamp string, currentTime time.Time, ttl time.Duration) bool {
	parsed, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(timestamp))
	if errorValue != nil {
		return false
	}
	return currentTime.Sub(parsed) >= 0 && currentTime.Sub(parsed) < ttl
}

func readCalendarHolidaySourceWithRunner(
	ctx context.Context,
	queryRunner *sql.DB,
	provider string,
	sourceKey string,
) (calendarHolidaySourceState, bool, error) {
	var source calendarHolidaySourceState
	errorValue := queryRunner.QueryRowContext(ctx, `
SELECT provider, source_key, last_synced_at, last_error
FROM calendar_holiday_sources
WHERE provider = ? AND source_key = ?`, provider, sourceKey).Scan(
		&source.Provider,
		&source.SourceKey,
		&source.LastSyncedAt,
		&source.LastError,
	)
	if errorValue == sql.ErrNoRows {
		return calendarHolidaySourceState{}, false, nil
	}
	return source, errorValue == nil, errorValue
}
