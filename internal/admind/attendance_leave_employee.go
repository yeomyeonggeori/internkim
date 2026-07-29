package admind

import (
	"net/http"
	"strings"
)

func (service *Service) attendanceLeaveEmployeeForRequest(
	request *http.Request,
	email string,
) attendanceLeaveEmployee {
	normalizedEmail := normalizeAttendanceLeaveEmail(email)
	employee := attendanceLeaveEmployee{Email: normalizedEmail}
	records, found := service.attendanceUserRecordsForMembers(request)
	if !found {
		return employee
	}
	for _, record := range records {
		if normalizeAttendanceLeaveEmail(record.Email) != normalizedEmail {
			continue
		}
		employee.UserID = strings.TrimSpace(record.UserID)
		employee.HireDate = strings.TrimSpace(record.HireDate)
		return employee
	}
	return employee
}

func attendanceLeaveEmployeeFromMember(member attendanceMember) attendanceLeaveEmployee {
	return attendanceLeaveEmployee{
		Email:    normalizeAttendanceLeaveEmail(member.Email),
		UserID:   strings.TrimSpace(member.UserID),
		HireDate: strings.TrimSpace(member.HireDate),
	}
}
