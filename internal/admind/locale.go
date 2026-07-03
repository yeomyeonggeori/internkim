package admind

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type adminLocaleResponse struct {
	Locale string `json:"locale"`
}

type adminLocaleUpdateRequest struct {
	Locale string `json:"locale"`
}

type localizedAdminText struct {
	FlowOpen                  string
	FlowEntryMessage          string
	CalendarOpen              string
	CalendarReminder          string
	Time                      string
	Location                  string
	People                    string
	Note                      string
	AttendanceOpen            string
	AttendanceEntryMessage    string
	AttendanceEntryText       string
	AttendanceClockIn         string
	AttendanceClockOut        string
	AttendanceCanceled        string
	AttendanceClockInTooltip  string
	AttendanceClockOutTooltip string
}

func (service *Service) writeAdminLocale(responseWriter http.ResponseWriter) {
	service.writeJSON(responseWriter, adminLocaleResponse{Locale: service.adminLocale()})
}

func (service *Service) updateAdminLocale(responseWriter http.ResponseWriter, request *http.Request) {
	var payload adminLocaleUpdateRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	locale := normalizeAdminLocale(payload.Locale)
	if errorValue := os.MkdirAll(service.Configuration.StateDirectory, 0o700); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue := os.WriteFile(service.adminLocalePath(), []byte(locale), 0o600); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, adminLocaleResponse{Locale: locale})
}

func (service *Service) adminLocale() string {
	return normalizeAdminLocale(readTrimmedFile(service.adminLocalePath()))
}

func (service *Service) adminText() localizedAdminText {
	return localizedAdminTextForLocale(service.adminLocale())
}

func (service *Service) adminLocalePath() string {
	return filepath.Join(service.Configuration.StateDirectory, "admin-locale")
}

func normalizeAdminLocale(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "en") {
		return "en"
	}
	return "ko"
}

func localizedAdminTextForLocale(locale string) localizedAdminText {
	if normalizeAdminLocale(locale) == "en" {
		return localizedAdminText{
			FlowOpen:                  "Open Flow",
			FlowEntryMessage:          "View, request, and organize this week's work in Flow.",
			CalendarOpen:              "Open Calendar",
			CalendarReminder:          "Calendar reminder",
			Time:                      "Time",
			Location:                  "Location",
			People:                    "People",
			Note:                      "Note",
			AttendanceOpen:            "Open Attendance",
			AttendanceEntryMessage:    "Clock in/out record",
			AttendanceEntryText:       "Use separate buttons for clock-in and clock-out records.",
			AttendanceClockIn:         "Clock in",
			AttendanceClockOut:        "Clock out",
			AttendanceCanceled:        "canceled",
			AttendanceClockInTooltip:  "Record clock-in.",
			AttendanceClockOutTooltip: "Record clock-out.",
		}
	}
	return localizedAdminText{
		FlowOpen:                  "업무 열기",
		FlowEntryMessage:          "업무에서 이번 주 일을 보고, 요청하고, 정리합니다.",
		CalendarOpen:              "일정 열기",
		CalendarReminder:          "일정 알림",
		Time:                      "시간",
		Location:                  "장소",
		People:                    "대상",
		Note:                      "메모",
		AttendanceOpen:            "근태 열기",
		AttendanceEntryMessage:    "출퇴근 기록",
		AttendanceEntryText:       "출근과 퇴근 버튼을 구분해서 기록합니다.",
		AttendanceClockIn:         "출근",
		AttendanceClockOut:        "퇴근",
		AttendanceCanceled:        "취소",
		AttendanceClockInTooltip:  "출근을 기록합니다.",
		AttendanceClockOutTooltip: "퇴근을 기록합니다.",
	}
}
