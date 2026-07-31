package admind

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCalendarHolidaySchemaIsIdempotent(t *testing.T) {
	service := newCalendarTestService(t)
	for range 2 {
		database, errorValue := service.openCalendarDatabase(context.Background())
		if errorValue != nil {
			t.Fatalf("open calendar database: %v", errorValue)
		}
		database.Close()
	}
}

func TestCalendarHolidaysUseCachedCountryAPIResponseWithConnectedGoogleAccount(t *testing.T) {
	service := newCalendarTestService(t)
	seedGoogleCalendarListAccount(t, service)
	var requestCount atomic.Int64
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestCount.Add(1)
		year := "2026"
		if strings.Contains(request.URL.Path, "/2027/") {
			year = "2027"
		} else if !strings.Contains(request.URL.Path, "/2026/") {
			t.Fatalf("unexpected Nager request path = %q", request.URL.Path)
		}
		body := strings.ReplaceAll(
			`[{"date":"YEAR-01-01","localName":"새해 첫날","name":"New Year's Day","countryCode":"KR","types":["Public"]}]`,
			"YEAR",
			year,
		)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}

	first := requestCalendarHolidaysForLocaleTest(t, service, workspaceLanguageKorean)
	second := requestCalendarHolidaysForLocaleTest(t, service, workspaceLanguageKorean)

	if first.Source != calendarHolidaySourceAPI || second.Source != calendarHolidaySourceAPI {
		t.Fatalf("sources = %q, %q", first.Source, second.Source)
	}
	if len(first.Holidays) != 1 || first.Holidays[0].Title != "새해 첫날" || !first.Holidays[0].ReadOnly {
		t.Fatalf("first holidays = %#v", first.Holidays)
	}
	if requestCount.Load() != calendarHolidayPreloadYears {
		t.Fatalf("API request count = %d, want %d preload years", requestCount.Load(), calendarHolidayPreloadYears)
	}
}

func TestCalendarHolidayRefreshPreloadsCurrentAndNextYearForChangedCountry(t *testing.T) {
	service := newCalendarTestService(t)
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{
		CountryCode: "US",
		Language:    workspaceLanguageEnglish,
	}); errorValue != nil {
		t.Fatalf("write workspace settings: %v", errorValue)
	}
	requestPaths := []string{}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestPaths = append(requestPaths, request.URL.Path)
		year := "2026"
		if strings.Contains(request.URL.Path, "/2027/") {
			year = "2027"
		}
		body := strings.ReplaceAll(
			`[{"date":"YEAR-07-04","localName":"Independence Day","name":"Independence Day","countryCode":"US","types":["Public"]}]`,
			"YEAR",
			year,
		)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}

	currentTime := time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
	if errorValue := service.refreshCalendarHolidayCache(context.Background(), currentTime); errorValue != nil {
		t.Fatalf("refresh calendar holidays: %v", errorValue)
	}

	if len(requestPaths) != calendarHolidayPreloadYears ||
		!strings.Contains(requestPaths[0], "/2026/US") ||
		!strings.Contains(requestPaths[1], "/2027/US") {
		t.Fatalf("holiday request paths = %#v", requestPaths)
	}
	holidays, errorValue := service.readCalendarHolidays(
		context.Background(),
		calendarHolidaySourceAPI,
		"US",
		workspaceLanguageEnglish,
		"2026-01-01",
		"2028-01-01",
	)
	if errorValue != nil {
		t.Fatalf("read calendar holidays: %v", errorValue)
	}
	if len(holidays) != 2 {
		t.Fatalf("holidays = %#v", holidays)
	}
}

func TestWorkspaceCountryUpdateRefreshesHolidaysWithoutMattermost(t *testing.T) {
	service := newCalendarTestService(t)
	holidayRequestPaths := []string{}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body string
		switch {
		case request.URL.Path == "/api/v3/AvailableCountries":
			body = `[{"countryCode":"KR","name":"South Korea"},{"countryCode":"US","name":"United States"}]`
		case strings.Contains(request.URL.Path, "/api/v3/PublicHolidays/"):
			holidayRequestPaths = append(holidayRequestPaths, request.URL.Path)
			segments := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
			year := segments[len(segments)-2]
			body = strings.ReplaceAll(
				`[{"date":"YEAR-07-04","localName":"Independence Day","name":"Independence Day","countryCode":"US","types":["Public"]}]`,
				"YEAR",
				year,
			)
		default:
			t.Fatalf("unexpected remote request: %s", request.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}
	request := httptest.NewRequest(
		http.MethodPut,
		"/admin/api/workspace-settings",
		strings.NewReader(`{"countryCode":"US","timeZone":"America/New_York","language":"ko","callingCode":"1"}`),
	)
	recorder := httptest.NewRecorder()

	service.updateWorkspaceSettings(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if len(holidayRequestPaths) != calendarHolidayPreloadYears {
		t.Fatalf("holiday request paths = %#v", holidayRequestPaths)
	}
	settings, errorValue := service.readWorkspaceSettings()
	if errorValue != nil {
		t.Fatalf("read workspace settings: %v", errorValue)
	}
	if settings.CountryCode != "US" {
		t.Fatalf("workspace country = %q", settings.CountryCode)
	}
}

func TestWorkspaceSettingsUpdateDoesNotRequireHolidayAPIWhenCountryIsUnchanged(t *testing.T) {
	service, _, _ := newWorkspaceSettingsMattermostTestService(t, false)
	originalTransport := service.HTTPClient.Transport
	var holidayRequestCount atomic.Int64
	service.HTTPClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "date.nager.at" {
			holidayRequestCount.Add(1)
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader("unavailable")),
				Request:    request,
			}, nil
		}
		return originalTransport.RoundTrip(request)
	})
	request := httptest.NewRequest(
		http.MethodPut,
		"/admin/api/workspace-settings",
		strings.NewReader(`{"countryCode":"KR","timeZone":"Asia/Seoul","language":"ko","callingCode":"82"}`),
	)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if holidayRequestCount.Load() != 0 {
		t.Fatalf("holiday request count = %d", holidayRequestCount.Load())
	}
}

func TestCalendarHolidaysServeStoredResponseWhenRequestRefreshFails(t *testing.T) {
	service := newCalendarTestService(t)
	currentTime := time.Now().UTC()
	workspaceLocation, _ := service.workspaceTimeLocation()
	localCurrentTime := currentTime.In(workspaceLocation)
	currentYear := localCurrentTime.Year()
	sourceKey := "KR:" + strconv.Itoa(currentYear) + ":" + workspaceLanguageKorean
	staleTime := time.Date(
		localCurrentTime.Year(),
		localCurrentTime.Month(),
		1,
		0,
		0,
		0,
		0,
		workspaceLocation,
	).Add(-time.Hour).UTC()
	if errorValue := service.replaceCalendarHolidaySnapshot(
		context.Background(),
		calendarHolidaySourceAPI,
		sourceKey,
		[]storedCalendarHoliday{{
			Source:      calendarHolidaySourceAPI,
			SourceKey:   sourceKey,
			ExternalID:  strconv.Itoa(currentYear) + "-01-01:새해 첫날",
			CountryCode: "KR",
			Title:       "새해 첫날",
			Date:        strconv.Itoa(currentYear) + "-01-01",
		}},
		staleTime,
	); errorValue != nil {
		t.Fatalf("store cached holiday: %v", errorValue)
	}
	for year := currentYear; year < currentYear+calendarHolidayPreloadYears; year += 1 {
		for _, locale := range [...]string{workspaceLanguageKorean, workspaceLanguageEnglish} {
			if errorValue := service.upsertCalendarHolidaySourceState(context.Background(), calendarHolidaySourceState{
				Provider:     calendarHolidayProviderNager,
				SourceKey:    calendarHolidaySourceKey("KR", year, locale),
				LastSyncedAt: staleTime.Format(time.RFC3339),
			}); errorValue != nil {
				t.Fatalf("store stale holiday state: %v", errorValue)
			}
		}
	}
	var requestCount atomic.Int64
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestCount.Add(1)
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("unavailable")),
			Request:    request,
		}, nil
	})}

	first := requestCalendarHolidaysForYearLocaleTest(t, service, currentYear, workspaceLanguageKorean)
	second := requestCalendarHolidaysForYearLocaleTest(t, service, currentYear, workspaceLanguageKorean)

	if len(first.Holidays) != 1 || first.Holidays[0].Title != "새해 첫날" {
		t.Fatalf("first holidays = %#v", first.Holidays)
	}
	if len(second.Holidays) != 1 || second.Holidays[0].Title != "새해 첫날" {
		t.Fatalf("second holidays = %#v", second.Holidays)
	}
	if requestCount.Load() != calendarHolidayMaximumAttempts {
		t.Fatalf("API request count = %d, want %d", requestCount.Load(), calendarHolidayMaximumAttempts)
	}
}

func TestCalendarHolidaysReturnBadGatewayWithoutCache(t *testing.T) {
	service := newCalendarTestService(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("unavailable")),
			Request:    request,
		}, nil
	})}
	currentYear := time.Now().UTC().Year()
	request := httptest.NewRequest(
		http.MethodGet,
		"/calendar/api/holidays?startISO="+time.Date(currentYear, time.January, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)+
			"&endISO="+time.Date(currentYear+1, time.January, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		nil,
	)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.serveCalendarHolidays(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCalendarHolidaysSkipNonPublicCountryAPIEntries(t *testing.T) {
	service := newCalendarTestService(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `[
			{"date":"2026-02-12","localName":"Lincoln's Birthday","name":"Lincoln's Birthday","countryCode":"US","types":["Observance"]},
			{"date":"2026-07-04","localName":"Independence Day","name":"Independence Day","countryCode":"US","types":["Public"]}
		]`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}

	holidays, errorValue := service.fetchNagerCalendarHolidays(context.Background(), "US", 2026)

	if errorValue != nil {
		t.Fatalf("fetch holidays: %v", errorValue)
	}
	if len(holidays) != 1 || holidays[0].Name != "Independence Day" {
		t.Fatalf("holidays = %#v", holidays)
	}
}

func TestCalendarHolidayCountriesUseCachedAPIResponse(t *testing.T) {
	service := newCalendarTestService(t)
	var requestCount atomic.Int64
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestCount.Add(1)
		if request.URL.Path != "/api/v3/AvailableCountries" {
			t.Fatalf("unexpected Nager request path = %q", request.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`[{"countryCode":"KR","name":"South Korea"},{"countryCode":"US","name":"United States"}]`)),
			Request:    request,
		}, nil
	})}

	first, firstError := service.ensureCalendarHolidayCountries(context.Background(), time.Now().UTC())
	second, secondError := service.ensureCalendarHolidayCountries(context.Background(), time.Now().UTC())

	if firstError != nil || secondError != nil {
		t.Fatalf("country errors = %v, %v", firstError, secondError)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("countries = %#v, %#v", first, second)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("API request count = %d, want one cached request", requestCount.Load())
	}
}

func TestCalendarHolidayCountriesKeepCacheWhenAPIResponseIsEmpty(t *testing.T) {
	service := newCalendarTestService(t)
	currentTime := time.Now().UTC()
	cachedCountries := []calendarHolidayCountry{{CountryCode: "KR", Name: "South Korea"}}
	if errorValue := service.replaceCalendarHolidayCountries(context.Background(), cachedCountries, currentTime.Add(-25*time.Hour)); errorValue != nil {
		t.Fatalf("store cached countries: %v", errorValue)
	}
	if errorValue := service.upsertCalendarHolidaySourceState(context.Background(), calendarHolidaySourceState{
		Provider:     calendarHolidayProviderNager,
		SourceKey:    calendarHolidayCountriesSourceKey,
		LastSyncedAt: currentTime.Add(-25 * time.Hour).Format(time.RFC3339),
	}); errorValue != nil {
		t.Fatalf("store stale country state: %v", errorValue)
	}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`[]`)),
			Request:    request,
		}, nil
	})}

	countries, errorValue := service.ensureCalendarHolidayCountries(context.Background(), currentTime)

	if errorValue != nil {
		t.Fatalf("ensure countries: %v", errorValue)
	}
	if len(countries) != 1 || countries[0].CountryCode != "KR" {
		t.Fatalf("countries = %#v", countries)
	}
	storedCountries, errorValue := service.readCalendarHolidayCountries(context.Background())
	if errorValue != nil {
		t.Fatalf("read cached countries: %v", errorValue)
	}
	if len(storedCountries) != 1 || storedCountries[0].CountryCode != "KR" {
		t.Fatalf("stored countries = %#v", storedCountries)
	}
}

func TestCalendarHolidaysUseEnglishNameForEnglishLocale(t *testing.T) {
	service := newCalendarTestService(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		year := "2026"
		if strings.Contains(request.URL.Path, "/2027/") {
			year = "2027"
		}
		body := strings.ReplaceAll(
			`[{"date":"YEAR-01-01","localName":"새해 첫날","name":"New Year's Day","countryCode":"KR","types":["Public"]}]`,
			"YEAR",
			year,
		)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}

	response := requestCalendarHolidaysForLocaleTest(t, service, workspaceLanguageEnglish)

	if len(response.Holidays) != 1 || response.Holidays[0].Title != "New Year's Day" {
		t.Fatalf("holidays = %#v", response.Holidays)
	}
}

func TestCalendarHolidayTitleUsesLocalNameOnlyForMatchingCountryLanguage(t *testing.T) {
	testCases := []struct {
		name        string
		countryCode string
		locale      string
		expected    string
	}{
		{name: "Korea in Korean", countryCode: "KR", locale: workspaceLanguageKorean, expected: "광복절"},
		{name: "Korea in English", countryCode: "KR", locale: workspaceLanguageEnglish, expected: "Liberation Day"},
		{name: "United States in Korean", countryCode: "US", locale: workspaceLanguageKorean, expected: "Liberation Day"},
		{name: "United States in English", countryCode: "US", locale: workspaceLanguageEnglish, expected: "Liberation Day"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			title := calendarHolidayTitle(testCase.countryCode, testCase.locale, "광복절", "Liberation Day")
			if title != testCase.expected {
				t.Fatalf("title = %q, want %q", title, testCase.expected)
			}
		})
	}
}

func requestCalendarHolidaysForLocaleTest(t *testing.T, service *Service, locale string) calendarHolidaysResponse {
	t.Helper()
	return requestCalendarHolidaysForYearLocaleTest(t, service, 2026, locale)
}

func requestCalendarHolidaysForYearLocaleTest(t *testing.T, service *Service, year int, locale string) calendarHolidaysResponse {
	t.Helper()
	startTime := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	request := httptest.NewRequest(
		http.MethodGet,
		"/calendar/api/holidays?startISO="+startTime.Format(time.RFC3339)+
			"&endISO="+startTime.AddDate(1, 0, 0).Format(time.RFC3339)+
			"&locale="+locale,
		nil,
	)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()
	service.serveCalendarHolidays(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response calendarHolidaysResponse
	if errorValue := json.NewDecoder(recorder.Body).Decode(&response); errorValue != nil {
		t.Fatalf("decode holidays: %v", errorValue)
	}
	return response
}
