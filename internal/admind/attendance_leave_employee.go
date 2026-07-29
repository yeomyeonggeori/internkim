package admind

import (
	"fmt"
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

func (service *Service) attendanceLeaveEmployeeWithHireDateForRequest(
	request *http.Request,
	email string,
) (attendanceLeaveEmployee, error) {
	employee := service.attendanceLeaveEmployeeForRequest(request, email)
	profiles, errorValue := service.readOrganizationProfiles(request.Context())
	if errorValue != nil {
		return attendanceLeaveEmployee{}, fmt.Errorf(
			"read organization profiles for attendance leave employee: %w",
			errorValue,
		)
	}
	profilesByUserID, profilesByEmail := organizationProfileIndexes(profiles)
	profile, found := organizationProfileForUser(
		adminUserMutation{UserID: employee.UserID, Email: employee.Email},
		profilesByUserID,
		profilesByEmail,
	)
	if found {
		employee.HireDate = strings.TrimSpace(profile.HireDate)
	}
	return employee, nil
}

func attendanceLeaveEmployeeFromMember(member attendanceMember) attendanceLeaveEmployee {
	return attendanceLeaveEmployee{
		Email:    normalizeAttendanceLeaveEmail(member.Email),
		UserID:   strings.TrimSpace(member.UserID),
		HireDate: strings.TrimSpace(member.HireDate),
	}
}

func ensureAttendanceLeaveEmployeeCanUseType(
	employee attendanceLeaveEmployee,
	leaveType attendanceLeaveType,
) error {
	if strings.TrimSpace(employee.HireDate) == "" &&
		attendanceLeaveTypeRequiresHireDate(leaveType) {
		return errAttendanceLeaveHireDateRequired
	}
	return nil
}
