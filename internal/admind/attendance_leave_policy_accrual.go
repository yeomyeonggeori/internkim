package admind

import (
	"fmt"
	"net/http"
	"time"
)

func (service *Service) synchronizeAttendanceLeavePolicyAccrualsBeforeUpdate(
	request *http.Request,
	policy attendanceLeavePolicy,
	now time.Time,
) error {
	records, found := service.attendanceUserRecordsForMembers(request)
	if !found {
		return fmt.Errorf("attendance employees are unavailable")
	}
	for _, member := range attendanceMembersFromAdminUserRecords(records) {
		if errorValue := service.synchronizeAttendanceLeaveAccruals(
			request.Context(),
			attendanceLeaveEmployeeFromMember(member),
			policy,
			now,
		); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
