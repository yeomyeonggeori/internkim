package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
)

const calendarHolidayCountriesSourceKey = "__countries__"

type calendarHolidayCountry struct {
	CountryCode string `json:"countryCode"`
	Name        string `json:"name"`
}

type calendarHolidayCountriesResponse struct {
	Countries []calendarHolidayCountry `json:"countries"`
}

func (service *Service) serveCalendarHolidayCountries(responseWriter http.ResponseWriter, request *http.Request) {
	countries, errorValue := service.ensureCalendarHolidayCountries(request.Context(), time.Now().UTC())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, calendarHolidayCountriesResponse{Countries: countries})
}

func (service *Service) ensureCalendarHolidayCountries(ctx context.Context, currentTime time.Time) ([]calendarHolidayCountry, error) {
	state, found, errorValue := service.readCalendarHolidaySource(ctx, calendarHolidayProviderNager, calendarHolidayCountriesSourceKey)
	if errorValue != nil {
		return nil, errorValue
	}
	cachedCountries, errorValue := service.readCalendarHolidayCountries(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	if found && len(cachedCountries) > 0 && calendarHolidaySourceIsFresh(state.LastSyncedAt, currentTime, calendarHolidayCacheTTL) {
		return cachedCountries, nil
	}
	countries, errorValue := service.fetchNagerCalendarHolidayCountriesWithRetry(ctx)
	if errorValue != nil {
		if len(cachedCountries) > 0 {
			slog.WarnContext(ctx, "holiday country refresh failed; serving cached countries", "error", errorValue)
			return cachedCountries, nil
		}
		return nil, errorValue
	}
	if errorValue := service.replaceCalendarHolidayCountries(ctx, countries, currentTime); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := service.upsertCalendarHolidaySourceState(ctx, calendarHolidaySourceState{
		Provider:     calendarHolidayProviderNager,
		SourceKey:    calendarHolidayCountriesSourceKey,
		LastSyncedAt: currentTime.UTC().Format(time.RFC3339),
	}); errorValue != nil {
		return nil, errorValue
	}
	return countries, nil
}

func (service *Service) fetchNagerCalendarHolidayCountriesWithRetry(ctx context.Context) ([]calendarHolidayCountry, error) {
	var lastError error
	for attempt := 0; attempt < calendarHolidayMaximumAttempts; attempt += 1 {
		countries, errorValue := service.fetchNagerCalendarHolidayCountries(ctx)
		if errorValue == nil {
			return countries, nil
		}
		lastError = errorValue
		slog.WarnContext(ctx, "holiday country API request failed", "attempt", attempt+1, "error", errorValue)
		if attempt+1 < calendarHolidayMaximumAttempts {
			time.Sleep(calendarHolidayRetryDelay(attempt))
		}
	}
	return nil, fmt.Errorf("fetch supported holiday countries: %w", lastError)
}

func (service *Service) fetchNagerCalendarHolidayCountries(ctx context.Context) ([]calendarHolidayCountry, error) {
	requestURL := nagerDateAPIBaseURL + "/AvailableCountries"
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
	body, errorValue := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if errorValue != nil {
		return nil, errorValue
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("nager country API status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var countries []calendarHolidayCountry
	if errorValue := json.Unmarshal(body, &countries); errorValue != nil {
		return nil, errorValue
	}
	seen := make(map[string]struct{}, len(countries))
	for index := range countries {
		countries[index].CountryCode = strings.ToUpper(strings.TrimSpace(countries[index].CountryCode))
		countries[index].Name = strings.TrimSpace(countries[index].Name)
		if errorValue := validateWorkspaceCountryCode(countries[index].CountryCode); errorValue != nil {
			return nil, fmt.Errorf("nager country code %q: %w", countries[index].CountryCode, errorValue)
		}
		if countries[index].Name == "" {
			return nil, fmt.Errorf("nager country name is required for %s", countries[index].CountryCode)
		}
		if _, duplicate := seen[countries[index].CountryCode]; duplicate {
			return nil, fmt.Errorf("nager returned duplicate country %s", countries[index].CountryCode)
		}
		seen[countries[index].CountryCode] = struct{}{}
	}
	slices.SortFunc(countries, func(first, second calendarHolidayCountry) int {
		return strings.Compare(first.CountryCode, second.CountryCode)
	})
	return countries, nil
}

func (service *Service) readCalendarHolidayCountries(ctx context.Context) ([]calendarHolidayCountry, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT country_code, name
FROM calendar_holiday_countries
ORDER BY country_code`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	countries := []calendarHolidayCountry{}
	for rows.Next() {
		var country calendarHolidayCountry
		if errorValue := rows.Scan(&country.CountryCode, &country.Name); errorValue != nil {
			return nil, errorValue
		}
		countries = append(countries, country)
	}
	return countries, rows.Err()
}

func (service *Service) replaceCalendarHolidayCountries(ctx context.Context, countries []calendarHolidayCountry, fetchedAt time.Time) error {
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
	if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM calendar_holiday_countries"); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	for _, country := range countries {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO calendar_holiday_countries (country_code, name, fetched_at)
VALUES (?, ?, ?)`,
			country.CountryCode,
			country.Name,
			fetchedAt.UTC().Format(time.RFC3339),
		); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
	}
	return transaction.Commit()
}

func calendarHolidayCountryIsSupported(countries []calendarHolidayCountry, countryCode string) bool {
	normalizedCountryCode := strings.ToUpper(strings.TrimSpace(countryCode))
	return slices.ContainsFunc(countries, func(country calendarHolidayCountry) bool {
		return country.CountryCode == normalizedCountryCode
	})
}
