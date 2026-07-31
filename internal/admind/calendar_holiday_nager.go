package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

const nagerDateAPIBaseURL = "https://date.nager.at/api/v3"

type nagerDateHoliday struct {
	Date        string   `json:"date"`
	LocalName   string   `json:"localName"`
	Name        string   `json:"name"`
	CountryCode string   `json:"countryCode"`
	Types       []string `json:"types"`
}

func (service *Service) refreshNagerCalendarHolidayYears(
	ctx context.Context,
	countryCode string,
	startYear int,
	endYear int,
	currentTime time.Time,
) error {
	for year := startYear; year <= endYear; year += 1 {
		holidays, errorValue := service.fetchNagerCalendarHolidaysWithRetry(ctx, countryCode, year)
		if errorValue != nil {
			service.recordNagerCalendarHolidayError(ctx, countryCode, year, errorValue)
			return errorValue
		}
		snapshots := make([]calendarHolidaySnapshot, 0, 2)
		states := make([]calendarHolidaySourceState, 0, 2)
		cachedHolidays := make(map[calendarHolidayCacheKey][]calendarHoliday, 2)
		for _, locale := range [...]string{workspaceLanguageKorean, workspaceLanguageEnglish} {
			key := newCalendarHolidayCacheKey(countryCode, locale, year)
			sourceKey := calendarHolidaySourceKey(key.CountryCode, key.Year, key.Locale)
			stored, transformError := storedNagerCalendarHolidays(key.CountryCode, key.Locale, sourceKey, holidays)
			if transformError != nil {
				return transformError
			}
			snapshots = append(snapshots, calendarHolidaySnapshot{SourceKey: sourceKey, Holidays: stored})
			states = append(states, calendarHolidaySourceState{
				Provider:     calendarHolidayProviderNager,
				SourceKey:    sourceKey,
				LastSyncedAt: currentTime.UTC().Format(time.RFC3339),
			})
			cachedHolidays[key] = calendarHolidaysFromStored(sourceKey, stored)
		}
		if errorValue := service.replaceCalendarHolidaySnapshots(
			ctx,
			calendarHolidaySourceAPI,
			snapshots,
			states,
			currentTime,
		); errorValue != nil {
			return errorValue
		}
		for key, yearHolidays := range cachedHolidays {
			service.writeCalendarHolidayMemoryCache(key, yearHolidays)
		}
	}
	return nil
}

func storedNagerCalendarHolidays(
	countryCode string,
	locale string,
	sourceKey string,
	holidays []nagerDateHoliday,
) ([]storedCalendarHoliday, error) {
	stored := make([]storedCalendarHoliday, 0, len(holidays))
	for _, holiday := range holidays {
		if _, parseError := time.Parse(time.DateOnly, holiday.Date); parseError != nil {
			return nil, fmt.Errorf("nager holiday date %q for %s: %w", holiday.Date, sourceKey, parseError)
		}
		title := calendarHolidayTitle(countryCode, locale, holiday.LocalName, holiday.Name)
		if title == "" {
			return nil, fmt.Errorf("nager holiday title is required for %s on %s", countryCode, holiday.Date)
		}
		stored = append(stored, storedCalendarHoliday{
			Source:      calendarHolidaySourceAPI,
			SourceKey:   sourceKey,
			ExternalID:  holiday.Date + ":" + title,
			CountryCode: countryCode,
			Title:       title,
			Date:        holiday.Date,
		})
	}
	return stored, nil
}

func calendarHolidaysFromStored(sourceKey string, stored []storedCalendarHoliday) []calendarHoliday {
	holidays := make([]calendarHoliday, 0, len(stored))
	for _, value := range stored {
		holidays = append(holidays, calendarHoliday{
			ID:          calendarHolidayID(calendarHolidaySourceAPI, sourceKey, value.ExternalID),
			Title:       value.Title,
			Date:        value.Date,
			Source:      calendarHolidaySourceAPI,
			CountryCode: value.CountryCode,
			ReadOnly:    true,
			Color:       calendarHolidayColor,
		})
	}
	return holidays
}

func (service *Service) recordNagerCalendarHolidayError(
	ctx context.Context,
	countryCode string,
	year int,
	syncError error,
) {
	for _, locale := range [...]string{workspaceLanguageKorean, workspaceLanguageEnglish} {
		sourceKey := calendarHolidaySourceKey(countryCode, year, locale)
		state, _, readError := service.readCalendarHolidaySource(ctx, calendarHolidayProviderNager, sourceKey)
		if readError != nil {
			slog.WarnContext(ctx, "calendar holiday source state read failed",
				"provider", calendarHolidayProviderNager,
				"source_key", sourceKey,
				"error", readError,
			)
			continue
		}
		state.Provider = calendarHolidayProviderNager
		state.SourceKey = sourceKey
		state.LastError = syncError.Error()
		if stateError := service.upsertCalendarHolidaySourceState(ctx, state); stateError != nil {
			slog.WarnContext(ctx, "calendar holiday source state update failed",
				"provider", calendarHolidayProviderNager,
				"source_key", sourceKey,
				"error", stateError,
			)
		}
	}
}

func calendarHolidaySourceKey(countryCode string, year int, locale string) string {
	return fmt.Sprintf("%s:%d:%s", strings.ToUpper(strings.TrimSpace(countryCode)), year, normalizeCalendarHolidayLocale(locale))
}

func calendarHolidayTitle(countryCode string, locale string, localName string, englishName string) string {
	if strings.EqualFold(strings.TrimSpace(countryCode), "KR") && locale == workspaceLanguageKorean {
		return firstNonEmpty(strings.TrimSpace(localName), strings.TrimSpace(englishName))
	}
	return firstNonEmpty(strings.TrimSpace(englishName), strings.TrimSpace(localName))
}

func (service *Service) fetchNagerCalendarHolidaysWithRetry(
	ctx context.Context,
	countryCode string,
	year int,
) ([]nagerDateHoliday, error) {
	var lastError error
	for attempt := 0; attempt < calendarHolidayMaximumAttempts; attempt += 1 {
		holidays, errorValue := service.fetchNagerCalendarHolidays(ctx, countryCode, year)
		if errorValue == nil {
			return holidays, nil
		}
		lastError = errorValue
		slog.WarnContext(ctx, "country holiday API request failed",
			"provider", calendarHolidayProviderNager,
			"country_code", countryCode,
			"year", year,
			"attempt", attempt+1,
			"error", errorValue,
		)
		if attempt+1 < calendarHolidayMaximumAttempts {
			time.Sleep(calendarHolidayRetryDelay(attempt))
		}
	}
	return nil, fmt.Errorf("fetch %s holidays for %d: %w", countryCode, year, lastError)
}

func (service *Service) fetchNagerCalendarHolidays(
	ctx context.Context,
	countryCode string,
	year int,
) ([]nagerDateHoliday, error) {
	requestURL := fmt.Sprintf(
		"%s/PublicHolidays/%d/%s",
		nagerDateAPIBaseURL,
		year,
		url.PathEscape(countryCode),
	)
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Accept", "application/json")
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	body, errorValue := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if errorValue != nil {
		return nil, errorValue
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("nager API status %d for %s/%d: %s",
			response.StatusCode,
			countryCode,
			year,
			strings.TrimSpace(string(body)),
		)
	}
	var holidays []nagerDateHoliday
	if errorValue := json.Unmarshal(body, &holidays); errorValue != nil {
		return nil, errorValue
	}
	publicHolidays := make([]nagerDateHoliday, 0, len(holidays))
	for _, holiday := range holidays {
		if !strings.EqualFold(strings.TrimSpace(holiday.CountryCode), countryCode) {
			return nil, fmt.Errorf("nager API returned country %q for %s/%d", holiday.CountryCode, countryCode, year)
		}
		if len(holiday.Types) > 0 && !slices.Contains(holiday.Types, "Public") {
			continue
		}
		publicHolidays = append(publicHolidays, holiday)
	}
	return publicHolidays, nil
}
