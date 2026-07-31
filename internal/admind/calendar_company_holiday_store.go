package admind

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const calendarHolidaySourceCompany = "company"

var errCalendarCompanyHolidayNotFound = errors.New("company holiday not found")

type calendarCompanyHoliday struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Date           string `json:"date"`
	RecursAnnually bool   `json:"recursAnnually"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type calendarCompanyHolidaysResponse struct {
	Holidays []calendarCompanyHoliday `json:"holidays"`
}

type calendarCompanyHolidayInput struct {
	Title          string `json:"title"`
	Date           string `json:"date"`
	RecursAnnually bool   `json:"recursAnnually"`
}

func normalizeCalendarCompanyHolidayInput(input calendarCompanyHolidayInput) (calendarCompanyHolidayInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Date = strings.TrimSpace(input.Date)
	if input.Title == "" {
		return calendarCompanyHolidayInput{}, fmt.Errorf("title is required")
	}
	if len([]rune(input.Title)) > 120 {
		return calendarCompanyHolidayInput{}, fmt.Errorf("title cannot exceed 120 characters")
	}
	date, errorValue := time.Parse(time.DateOnly, input.Date)
	if errorValue != nil || date.Format(time.DateOnly) != input.Date {
		return calendarCompanyHolidayInput{}, fmt.Errorf("date must use YYYY-MM-DD")
	}
	return input, nil
}

func (service *Service) listCalendarCompanyHolidays(ctx context.Context) ([]calendarCompanyHoliday, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, title, holiday_date, recurs_annually, created_at, updated_at
FROM calendar_company_holidays
ORDER BY holiday_date, title, id`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	holidays := make([]calendarCompanyHoliday, 0)
	for rows.Next() {
		var holiday calendarCompanyHoliday
		if errorValue := rows.Scan(
			&holiday.ID,
			&holiday.Title,
			&holiday.Date,
			&holiday.RecursAnnually,
			&holiday.CreatedAt,
			&holiday.UpdatedAt,
		); errorValue != nil {
			return nil, errorValue
		}
		holidays = append(holidays, holiday)
	}
	return holidays, rows.Err()
}

func (service *Service) createCalendarCompanyHoliday(
	ctx context.Context,
	input calendarCompanyHolidayInput,
	currentTime time.Time,
) (calendarCompanyHoliday, error) {
	normalized, errorValue := normalizeCalendarCompanyHolidayInput(input)
	if errorValue != nil {
		return calendarCompanyHoliday{}, errorValue
	}
	timestamp := currentTime.UTC().Format(time.RFC3339Nano)
	holiday := calendarCompanyHoliday{
		ID:             "company-holiday-" + randomHex(16),
		Title:          normalized.Title,
		Date:           normalized.Date,
		RecursAnnually: normalized.RecursAnnually,
		CreatedAt:      timestamp,
		UpdatedAt:      timestamp,
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarCompanyHoliday{}, errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO calendar_company_holidays (
	id, title, holiday_date, recurs_annually, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?)`,
		holiday.ID,
		holiday.Title,
		holiday.Date,
		holiday.RecursAnnually,
		holiday.CreatedAt,
		holiday.UpdatedAt,
	)
	return holiday, errorValue
}

func (service *Service) updateCalendarCompanyHoliday(
	ctx context.Context,
	holidayID string,
	input calendarCompanyHolidayInput,
	currentTime time.Time,
) (calendarCompanyHoliday, error) {
	normalized, errorValue := normalizeCalendarCompanyHolidayInput(input)
	if errorValue != nil {
		return calendarCompanyHoliday{}, errorValue
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarCompanyHoliday{}, errorValue
	}
	defer database.Close()
	timestamp := currentTime.UTC().Format(time.RFC3339Nano)
	result, errorValue := database.ExecContext(ctx, `
UPDATE calendar_company_holidays
SET title = ?, holiday_date = ?, recurs_annually = ?, updated_at = ?
WHERE id = ?`,
		normalized.Title,
		normalized.Date,
		normalized.RecursAnnually,
		timestamp,
		holidayID,
	)
	if errorValue != nil {
		return calendarCompanyHoliday{}, errorValue
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		return calendarCompanyHoliday{}, errorValue
	}
	if affectedRows == 0 {
		return calendarCompanyHoliday{}, errCalendarCompanyHolidayNotFound
	}
	var holiday calendarCompanyHoliday
	errorValue = database.QueryRowContext(ctx, `
SELECT id, title, holiday_date, recurs_annually, created_at, updated_at
FROM calendar_company_holidays
WHERE id = ?`, holidayID).Scan(
		&holiday.ID,
		&holiday.Title,
		&holiday.Date,
		&holiday.RecursAnnually,
		&holiday.CreatedAt,
		&holiday.UpdatedAt,
	)
	return holiday, errorValue
}

func (service *Service) deleteCalendarCompanyHoliday(ctx context.Context, holidayID string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	result, errorValue := database.ExecContext(ctx, "DELETE FROM calendar_company_holidays WHERE id = ?", holidayID)
	if errorValue != nil {
		return errorValue
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		return errorValue
	}
	if affectedRows == 0 {
		return errCalendarCompanyHolidayNotFound
	}
	return nil
}

func (service *Service) readCalendarCompanyHolidaysForRange(
	ctx context.Context,
	startTime time.Time,
	endTime time.Time,
) ([]calendarHoliday, error) {
	companyHolidays, errorValue := service.listCalendarCompanyHolidays(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	location, _ := service.workspaceTimeLocation()
	rangeStart := startTime.In(location)
	rangeEnd := endTime.In(location)
	startDateValue := rangeStart.Format(time.DateOnly)
	endDateValue := rangeEnd.Format(time.DateOnly)
	holidays := make([]calendarHoliday, 0)
	for _, companyHoliday := range companyHolidays {
		baseDate, _ := time.Parse(time.DateOnly, companyHoliday.Date)
		if !companyHoliday.RecursAnnually {
			date := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 0, 0, 0, 0, location)
			dateValue := date.Format(time.DateOnly)
			if dateValue >= startDateValue && dateValue < endDateValue {
				holidays = append(holidays, calendarHolidayFromCompanyHoliday(companyHoliday, date))
			}
			continue
		}
		for year := rangeStart.Year(); year <= rangeEnd.Year(); year += 1 {
			date := time.Date(year, baseDate.Month(), baseDate.Day(), 0, 0, 0, 0, location)
			if date.Month() != baseDate.Month() || date.Day() != baseDate.Day() {
				continue
			}
			dateValue := date.Format(time.DateOnly)
			if dateValue >= startDateValue && dateValue < endDateValue {
				holidays = append(holidays, calendarHolidayFromCompanyHoliday(companyHoliday, date))
			}
		}
	}
	sortCalendarHolidays(holidays)
	return holidays, nil
}

func calendarHolidayFromCompanyHoliday(companyHoliday calendarCompanyHoliday, date time.Time) calendarHoliday {
	dateValue := date.Format(time.DateOnly)
	return calendarHoliday{
		ID:       calendarHolidayID(calendarHolidaySourceCompany, companyHoliday.ID, dateValue),
		Title:    companyHoliday.Title,
		Date:     dateValue,
		Source:   calendarHolidaySourceCompany,
		ReadOnly: true,
		Color:    calendarHolidayColor,
	}
}

func sortCalendarHolidays(holidays []calendarHoliday) {
	sort.SliceStable(holidays, func(left int, right int) bool {
		if holidays[left].Date != holidays[right].Date {
			return holidays[left].Date < holidays[right].Date
		}
		if holidays[left].Title != holidays[right].Title {
			return holidays[left].Title < holidays[right].Title
		}
		return holidays[left].ID < holidays[right].ID
	})
}
