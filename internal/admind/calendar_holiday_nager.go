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

func (service *Service) syncNagerCalendarHolidays(
	ctx context.Context,
	countryCode string,
	locale string,
	startTime time.Time,
	endTime time.Time,
	currentTime time.Time,
) error {
	workspaceLocation, _ := service.workspaceTimeLocation()
	startYear := startTime.In(workspaceLocation).Year()
	endYear := endTime.In(workspaceLocation).AddDate(0, 0, -1).Year()
	for year := startYear; year <= endYear; year += 1 {
		sourceKey := fmt.Sprintf("%s:%d:%s", countryCode, year, locale)
		state, found, errorValue := service.readCalendarHolidaySource(ctx, calendarHolidayProviderNager, sourceKey)
		if errorValue != nil {
			return errorValue
		}
		if found && calendarHolidaySourceIsFresh(state.LastSyncedAt, currentTime, calendarHolidayCacheTTL) {
			continue
		}
		holidays, errorValue := service.fetchNagerCalendarHolidaysWithRetry(ctx, countryCode, year)
		if errorValue != nil {
			state.Provider = calendarHolidayProviderNager
			state.SourceKey = sourceKey
			state.LastError = errorValue.Error()
			if stateError := service.upsertCalendarHolidaySourceState(ctx, state); stateError != nil {
				slog.WarnContext(ctx, "calendar holiday source state update failed",
					"provider", calendarHolidayProviderNager,
					"source_key", sourceKey,
					"error", stateError,
				)
			}
			return errorValue
		}
		stored := make([]storedCalendarHoliday, 0, len(holidays))
		for _, holiday := range holidays {
			if _, parseError := time.Parse(time.DateOnly, holiday.Date); parseError != nil {
				return fmt.Errorf("nager holiday date %q for %s: %w", holiday.Date, sourceKey, parseError)
			}
			title := calendarHolidayTitle(countryCode, locale, holiday.LocalName, holiday.Name)
			if title == "" {
				return fmt.Errorf("nager holiday title is required for %s on %s", countryCode, holiday.Date)
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
		if errorValue := service.replaceCalendarHolidaySnapshot(
			ctx,
			calendarHolidaySourceAPI,
			sourceKey,
			stored,
			currentTime,
		); errorValue != nil {
			return errorValue
		}
		if errorValue := service.upsertCalendarHolidaySourceState(ctx, calendarHolidaySourceState{
			Provider:     calendarHolidayProviderNager,
			SourceKey:    sourceKey,
			LastSyncedAt: currentTime.UTC().Format(time.RFC3339),
		}); errorValue != nil {
			return errorValue
		}
	}
	return nil
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
