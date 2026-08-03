package admind

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	calendarHolidayProviderUnavailableCode = "HOLIDAY_PROVIDER_UNAVAILABLE"
	calendarHolidayRetryStateFileName      = "calendar-holiday-retry-state.json"
	calendarHolidayRetryStateVersion       = 1
)

var calendarHolidayProviderRetryDelays = [...]time.Duration{
	time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	time.Hour,
}

type calendarHolidayRetryKey struct {
	CountryCode string
	Year        int
}

type calendarHolidayRetryState struct {
	FailureCount int
	LastAttempt  time.Time
	NextRetryAt  time.Time
	LastError    string
}

type calendarHolidayPersistedRetryState struct {
	CountryCode  string    `json:"countryCode"`
	Year         int       `json:"year"`
	FailureCount int       `json:"failureCount"`
	LastAttempt  time.Time `json:"lastAttempt"`
	NextRetryAt  time.Time `json:"nextRetryAt"`
	LastError    string    `json:"lastError,omitempty"`
}

type calendarHolidayRetryStateDocument struct {
	Version int                                  `json:"version"`
	States  []calendarHolidayPersistedRetryState `json:"states"`
}

type calendarHolidayYearFailure struct {
	CountryCode string
	Year        int
	Error       string
	NextRetryAt time.Time
}

type calendarHolidayProviderRefreshError struct {
	Failures []calendarHolidayYearFailure
}

func (errorValue *calendarHolidayProviderRefreshError) Error() string {
	if len(errorValue.Failures) == 0 {
		return "calendar holiday provider refresh failed"
	}
	failure := errorValue.Failures[0]
	return fmt.Sprintf("refresh %s holidays for %d: %s", failure.CountryCode, failure.Year, failure.Error)
}

func newCalendarHolidayRetryKey(countryCode string, year int) calendarHolidayRetryKey {
	return calendarHolidayRetryKey{CountryCode: strings.ToUpper(strings.TrimSpace(countryCode)), Year: year}
}

func (service *Service) loadCalendarHolidayRetryStates() error {
	path := service.calendarHolidayRetryStatePath()
	documentBytes, errorValue := os.ReadFile(path)
	if os.IsNotExist(errorValue) {
		return nil
	}
	if errorValue != nil {
		return fmt.Errorf("read calendar holiday retry state %s: %w", path, errorValue)
	}
	var document calendarHolidayRetryStateDocument
	if errorValue := json.Unmarshal(documentBytes, &document); errorValue != nil {
		return fmt.Errorf("decode calendar holiday retry state %s: %w", path, errorValue)
	}
	if document.Version != calendarHolidayRetryStateVersion {
		return fmt.Errorf("decode calendar holiday retry state %s: unsupported version %d", path, document.Version)
	}
	states := make(map[calendarHolidayRetryKey]calendarHolidayRetryState, len(document.States))
	for _, persistedState := range document.States {
		key := newCalendarHolidayRetryKey(persistedState.CountryCode, persistedState.Year)
		if key.CountryCode == "" || key.Year <= 0 || persistedState.FailureCount < 0 {
			return fmt.Errorf("decode calendar holiday retry state %s: invalid country, year, or failure count", path)
		}
		states[key] = calendarHolidayRetryState{
			FailureCount: persistedState.FailureCount,
			LastAttempt:  persistedState.LastAttempt.UTC(),
			NextRetryAt:  persistedState.NextRetryAt.UTC(),
			LastError:    persistedState.LastError,
		}
	}
	service.calendarHolidayRetryStates = states
	return nil
}

func (service *Service) calendarHolidayRetryStatePath() string {
	return filepath.Join(service.Configuration.StateDirectory, calendarHolidayRetryStateFileName)
}

func (service *Service) persistCalendarHolidayRetryStates(states map[calendarHolidayRetryKey]calendarHolidayRetryState) error {
	document := calendarHolidayRetryStateDocument{
		Version: calendarHolidayRetryStateVersion,
		States:  make([]calendarHolidayPersistedRetryState, 0, len(states)),
	}
	for key, state := range states {
		document.States = append(document.States, calendarHolidayPersistedRetryState{
			CountryCode:  key.CountryCode,
			Year:         key.Year,
			FailureCount: state.FailureCount,
			LastAttempt:  state.LastAttempt,
			NextRetryAt:  state.NextRetryAt,
			LastError:    state.LastError,
		})
	}
	sort.Slice(document.States, func(leftIndex int, rightIndex int) bool {
		left := document.States[leftIndex]
		right := document.States[rightIndex]
		if left.CountryCode == right.CountryCode {
			return left.Year < right.Year
		}
		return left.CountryCode < right.CountryCode
	})
	documentBytes, errorValue := json.Marshal(document)
	if errorValue != nil {
		return fmt.Errorf("encode calendar holiday retry state: %w", errorValue)
	}
	path := service.calendarHolidayRetryStatePath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return fmt.Errorf("create calendar holiday retry state directory: %w", errorValue)
	}
	temporaryPath := path + ".tmp"
	if errorValue := os.WriteFile(temporaryPath, documentBytes, 0o600); errorValue != nil {
		return fmt.Errorf("write calendar holiday retry state %s: %w", temporaryPath, errorValue)
	}
	if errorValue := os.Rename(temporaryPath, path); errorValue != nil {
		return fmt.Errorf("replace calendar holiday retry state %s: %w", path, errorValue)
	}
	return nil
}

func cloneCalendarHolidayRetryStates(states map[calendarHolidayRetryKey]calendarHolidayRetryState) map[calendarHolidayRetryKey]calendarHolidayRetryState {
	cloned := make(map[calendarHolidayRetryKey]calendarHolidayRetryState, len(states)+1)
	for key, state := range states {
		cloned[key] = state
	}
	return cloned
}

func (service *Service) calendarHolidayRetryState(countryCode string, year int) calendarHolidayRetryState {
	service.calendarHolidayRetryMutex.RLock()
	defer service.calendarHolidayRetryMutex.RUnlock()
	return service.calendarHolidayRetryStates[newCalendarHolidayRetryKey(countryCode, year)]
}

func (service *Service) calendarHolidayRetryAllowed(countryCode string, year int, currentTime time.Time) bool {
	service.calendarHolidayRetryMutex.RLock()
	defer service.calendarHolidayRetryMutex.RUnlock()
	if service.calendarHolidayRetryLoadError != nil {
		return false
	}
	state := service.calendarHolidayRetryStates[newCalendarHolidayRetryKey(countryCode, year)]
	return state.NextRetryAt.IsZero() || !currentTime.Before(state.NextRetryAt)
}

func (service *Service) calendarHolidayRetryLoadErrorMessage() string {
	service.calendarHolidayRetryMutex.RLock()
	defer service.calendarHolidayRetryMutex.RUnlock()
	if service.calendarHolidayRetryLoadError == nil {
		return ""
	}
	return service.calendarHolidayRetryLoadError.Error()
}

func (service *Service) recordCalendarHolidayRefreshAttempt(countryCode string, year int, currentTime time.Time) error {
	service.calendarHolidayRetryMutex.Lock()
	defer service.calendarHolidayRetryMutex.Unlock()
	if service.calendarHolidayRetryLoadError != nil {
		return service.calendarHolidayRetryLoadError
	}
	states := cloneCalendarHolidayRetryStates(service.calendarHolidayRetryStates)
	key := newCalendarHolidayRetryKey(countryCode, year)
	state := states[key]
	state.LastAttempt = currentTime.UTC()
	states[key] = state
	if errorValue := service.persistCalendarHolidayRetryStates(states); errorValue != nil {
		return errorValue
	}
	service.calendarHolidayRetryStates = states
	return nil
}

func (service *Service) recordCalendarHolidayRefreshFailure(countryCode string, year int, currentTime time.Time, refreshError error) (calendarHolidayRetryState, error) {
	service.calendarHolidayRetryMutex.Lock()
	defer service.calendarHolidayRetryMutex.Unlock()
	if service.calendarHolidayRetryLoadError != nil {
		return calendarHolidayRetryState{}, service.calendarHolidayRetryLoadError
	}
	states := cloneCalendarHolidayRetryStates(service.calendarHolidayRetryStates)
	key := newCalendarHolidayRetryKey(countryCode, year)
	state := states[key]
	state.FailureCount += 1
	delayIndex := min(state.FailureCount-1, len(calendarHolidayProviderRetryDelays)-1)
	state.LastAttempt = currentTime.UTC()
	state.NextRetryAt = currentTime.UTC().Add(calendarHolidayProviderRetryDelays[delayIndex])
	state.LastError = refreshError.Error()
	states[key] = state
	if errorValue := service.persistCalendarHolidayRetryStates(states); errorValue != nil {
		return calendarHolidayRetryState{}, errorValue
	}
	service.calendarHolidayRetryStates = states
	return state, nil
}

func (service *Service) recordCalendarHolidayRefreshSuccess(countryCode string, year int, currentTime time.Time) error {
	service.calendarHolidayRetryMutex.Lock()
	defer service.calendarHolidayRetryMutex.Unlock()
	if service.calendarHolidayRetryLoadError != nil {
		return service.calendarHolidayRetryLoadError
	}
	states := cloneCalendarHolidayRetryStates(service.calendarHolidayRetryStates)
	states[newCalendarHolidayRetryKey(countryCode, year)] = calendarHolidayRetryState{
		LastAttempt: currentTime.UTC(),
	}
	if errorValue := service.persistCalendarHolidayRetryStates(states); errorValue != nil {
		return errorValue
	}
	service.calendarHolidayRetryStates = states
	return nil
}

func (service *Service) earliestCalendarHolidayRetry(failures []calendarHolidayYearFailure) time.Time {
	var earliest time.Time
	for _, failure := range failures {
		if failure.NextRetryAt.IsZero() {
			continue
		}
		if earliest.IsZero() || failure.NextRetryAt.Before(earliest) {
			earliest = failure.NextRetryAt
		}
	}
	return earliest
}
