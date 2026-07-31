package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCalendarCompanyHolidayCRUDAndAnnualExpansion(t *testing.T) {
	service := newCalendarTestService(t)
	currentTime := time.Date(2026, time.July, 31, 10, 0, 0, 0, time.UTC)
	created, errorValue := service.createCalendarCompanyHoliday(context.Background(), calendarCompanyHolidayInput{
		Title:          " 창립기념일 ",
		Date:           "2026-09-18",
		RecursAnnually: true,
	}, currentTime)
	if errorValue != nil {
		t.Fatalf("create company holiday: %v", errorValue)
	}
	if created.Title != "창립기념일" || created.Date != "2026-09-18" || !created.RecursAnnually {
		t.Fatalf("created company holiday = %#v", created)
	}

	location, _ := service.workspaceTimeLocation()
	holidays, errorValue := service.readCalendarCompanyHolidaysForRange(
		context.Background(),
		time.Date(2026, time.January, 1, 0, 0, 0, 0, location),
		time.Date(2028, time.January, 1, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatalf("read recurring company holidays: %v", errorValue)
	}
	if len(holidays) != 2 ||
		holidays[0].Date != "2026-09-18" ||
		holidays[1].Date != "2027-09-18" ||
		holidays[0].Source != calendarHolidaySourceCompany ||
		!holidays[0].ReadOnly {
		t.Fatalf("expanded company holidays = %#v", holidays)
	}

	updated, errorValue := service.updateCalendarCompanyHoliday(context.Background(), created.ID, calendarCompanyHolidayInput{
		Title:          "여름 전사 휴무",
		Date:           "2026-08-14",
		RecursAnnually: false,
	}, currentTime.Add(time.Hour))
	if errorValue != nil {
		t.Fatalf("update company holiday: %v", errorValue)
	}
	if updated.Title != "여름 전사 휴무" || updated.Date != "2026-08-14" || updated.RecursAnnually {
		t.Fatalf("updated company holiday = %#v", updated)
	}
	if errorValue := service.deleteCalendarCompanyHoliday(context.Background(), created.ID); errorValue != nil {
		t.Fatalf("delete company holiday: %v", errorValue)
	}
	if errorValue := service.deleteCalendarCompanyHoliday(context.Background(), created.ID); errorValue != errCalendarCompanyHolidayNotFound {
		t.Fatalf("second delete error = %v", errorValue)
	}
}

func TestCalendarCompanyHolidayAnnualLeapDayOnlyAppearsInLeapYears(t *testing.T) {
	service := newCalendarTestService(t)
	if _, errorValue := service.createCalendarCompanyHoliday(context.Background(), calendarCompanyHolidayInput{
		Title:          "윤년 휴일",
		Date:           "2024-02-29",
		RecursAnnually: true,
	}, time.Now()); errorValue != nil {
		t.Fatalf("create leap day holiday: %v", errorValue)
	}
	location, _ := service.workspaceTimeLocation()
	holidays, errorValue := service.readCalendarCompanyHolidaysForRange(
		context.Background(),
		time.Date(2025, time.January, 1, 0, 0, 0, 0, location),
		time.Date(2029, time.January, 1, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatalf("read leap day holidays: %v", errorValue)
	}
	if len(holidays) != 1 || holidays[0].Date != "2028-02-29" {
		t.Fatalf("leap day holidays = %#v", holidays)
	}
}

func TestCalendarHolidaysPreserveNationalAndCompanyHolidaysOnTheSameDate(t *testing.T) {
	service := newCalendarTestService(t)
	var requestCount atomic.Int64
	service.HTTPClient = calendarHolidayTestHTTPClient(t, "KR", &requestCount)
	if _, errorValue := service.createCalendarCompanyHoliday(context.Background(), calendarCompanyHolidayInput{
		Title:          "신년 전사 휴무",
		Date:           "2026-01-01",
		RecursAnnually: false,
	}, time.Now()); errorValue != nil {
		t.Fatalf("create company holiday: %v", errorValue)
	}

	response := requestCalendarHolidaysForYearLocaleTest(t, service, 2026, workspaceLanguageKorean)

	if len(response.Holidays) != 2 ||
		response.Holidays[0].Date != "2026-01-01" ||
		response.Holidays[1].Date != "2026-01-01" ||
		response.Holidays[0].Source == response.Holidays[1].Source {
		t.Fatalf("same-date holidays = %#v", response.Holidays)
	}
}

func TestCalendarCompanyHolidayAdminCRUDAndAuthorization(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	requestHoliday := func(method string, path string, body string, email string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Cf-Access-Authenticated-User-Email", email)
		response := httptest.NewRecorder()
		service.router().ServeHTTP(response, request)
		return response
	}

	createResponse := requestHoliday(
		http.MethodPost,
		"/admin/api/company-holidays",
		`{"title":"창립기념일","date":"2026-09-18","recursAnnually":true}`,
		"owner@example.com",
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("admin create status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}
	var created calendarCompanyHoliday
	if errorValue := json.NewDecoder(createResponse.Body).Decode(&created); errorValue != nil {
		t.Fatalf("decode created company holiday: %v", errorValue)
	}
	if created.ID == "" || created.Title != "창립기념일" {
		t.Fatalf("created company holiday = %#v", created)
	}

	deniedCreateResponse := requestHoliday(
		http.MethodPost,
		"/admin/api/company-holidays",
		`{"title":"운영 관리자 휴일","date":"2026-09-19","recursAnnually":false}`,
		"operator@example.com",
	)
	if deniedCreateResponse.Code != http.StatusForbidden {
		t.Fatalf("operations admin create status = %d body = %s", deniedCreateResponse.Code, deniedCreateResponse.Body.String())
	}
	holidays, errorValue := service.listCalendarCompanyHolidays(t.Context())
	if errorValue != nil {
		t.Fatalf("list company holidays after denied create: %v", errorValue)
	}
	if len(holidays) != 1 || holidays[0].ID != created.ID {
		t.Fatalf("company holidays after denied create = %#v", holidays)
	}

	holidayPath := "/admin/api/company-holidays/" + created.ID
	updateResponse := requestHoliday(
		http.MethodPut,
		holidayPath,
		`{"title":"전사 휴무일","date":"2026-10-02","recursAnnually":false}`,
		"owner@example.com",
	)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("admin update status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}
	var updated calendarCompanyHoliday
	if errorValue := json.NewDecoder(updateResponse.Body).Decode(&updated); errorValue != nil {
		t.Fatalf("decode updated company holiday: %v", errorValue)
	}
	if updated.ID != created.ID || updated.Title != "전사 휴무일" || updated.Date != "2026-10-02" || updated.RecursAnnually {
		t.Fatalf("updated company holiday = %#v", updated)
	}

	deniedUpdateResponse := requestHoliday(
		http.MethodPut,
		holidayPath,
		`{"title":"변경되면 안 됨","date":"2026-10-03","recursAnnually":true}`,
		"operator@example.com",
	)
	if deniedUpdateResponse.Code != http.StatusForbidden {
		t.Fatalf("operations admin update status = %d body = %s", deniedUpdateResponse.Code, deniedUpdateResponse.Body.String())
	}
	holidays, errorValue = service.listCalendarCompanyHolidays(t.Context())
	if errorValue != nil {
		t.Fatalf("list company holidays after denied update: %v", errorValue)
	}
	if len(holidays) != 1 || holidays[0].Title != updated.Title || holidays[0].Date != updated.Date || holidays[0].RecursAnnually {
		t.Fatalf("company holidays after denied update = %#v", holidays)
	}

	deniedDeleteResponse := requestHoliday(http.MethodDelete, holidayPath, "", "operator@example.com")
	if deniedDeleteResponse.Code != http.StatusForbidden {
		t.Fatalf("operations admin delete status = %d body = %s", deniedDeleteResponse.Code, deniedDeleteResponse.Body.String())
	}
	holidays, errorValue = service.listCalendarCompanyHolidays(t.Context())
	if errorValue != nil {
		t.Fatalf("list company holidays after denied delete: %v", errorValue)
	}
	if len(holidays) != 1 || holidays[0].ID != created.ID {
		t.Fatalf("company holidays after denied delete = %#v", holidays)
	}

	deleteResponse := requestHoliday(http.MethodDelete, holidayPath, "", "owner@example.com")
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("admin delete status = %d body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	holidays, errorValue = service.listCalendarCompanyHolidays(t.Context())
	if errorValue != nil {
		t.Fatalf("list company holidays after delete: %v", errorValue)
	}
	if len(holidays) != 0 {
		t.Fatalf("company holidays after delete = %#v", holidays)
	}
}

func TestCalendarCompanyHolidayInputValidation(t *testing.T) {
	testCases := []calendarCompanyHolidayInput{
		{Title: "", Date: "2026-09-18"},
		{Title: "회사 휴일", Date: "2026-02-29"},
		{Title: "회사 휴일", Date: "18-09-2026"},
	}
	for _, input := range testCases {
		if _, errorValue := normalizeCalendarCompanyHolidayInput(input); errorValue == nil {
			t.Fatalf("input should fail validation = %#v", input)
		}
	}
}
