package admind

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAttendanceLeaveRequestUpdateReplacesReservationAndAttachment(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-leave-request-update",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       2000,
			EffectiveOn:  "2027-01-01",
		},
		ExpiresOn: "2028-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	createRecorder := performAttendanceLeaveMultipartRequestWithAttachment(
		t,
		service,
		"/attendance/api/leave-requests",
		employee.Email,
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03","reason":"Original reason"}`,
		"original.pdf",
		[]byte("%PDF-1.7 original evidence"),
	)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Request attendanceLeaveRequestView `json:"request"`
	}
	if errorValue := json.NewDecoder(createRecorder.Body).Decode(&created); errorValue != nil {
		t.Fatal(errorValue)
	}
	if created.Request.Revision != 1 || len(created.Request.Attachments) != 1 {
		t.Fatalf("created request = %+v", created.Request)
	}
	removedAttachmentID := created.Request.Attachments[0].ID

	updateRecorder := performAttendanceLeaveMultipartRequestWithAttachment(
		t,
		service,
		"/attendance/api/leave-requests/"+created.Request.ID+"/update",
		employee.Email,
		fmt.Sprintf(
			`{"leaveTypeID":"annual","unit":"halfDay","startDate":"2027-05-04","partialPeriod":"afternoon","reason":"Updated reason","revision":1,"removedAttachmentIDs":[%q]}`,
			removedAttachmentID,
		),
		"replacement.pdf",
		[]byte("%PDF-1.7 replacement evidence"),
	)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated struct {
		Request attendanceLeaveRequestView `json:"request"`
	}
	if errorValue := json.NewDecoder(updateRecorder.Body).Decode(&updated); errorValue != nil {
		t.Fatal(errorValue)
	}
	if updated.Request.Status != attendanceLeaveRequestStatusPending ||
		updated.Request.Unit != attendanceWorkScheduleHalfDay ||
		updated.Request.Reason != "Updated reason" ||
		updated.Request.Revision != 2 ||
		!updated.Request.CanEdit ||
		len(updated.Request.Attachments) != 1 ||
		updated.Request.Attachments[0].FileName != "replacement.pdf" {
		t.Fatalf("updated request = %+v", updated.Request)
	}

	dashboard := readAttendanceLeaveDashboardForTest(t, service, employee.Email)
	if dashboard.Summary.AvailableMilliDays != 1500 ||
		dashboard.Summary.ReservedMilliDays != 500 {
		t.Fatalf("summary = %+v", dashboard.Summary)
	}
	if len(dashboard.Requests) != 1 ||
		dashboard.Requests[0].StartDate != "2027-05-04" ||
		dashboard.Requests[0].StartTime != "14:00" ||
		dashboard.Requests[0].EndTime != "18:00" {
		t.Fatalf("dashboard requests = %+v", dashboard.Requests)
	}

	removedAttachmentRecorder := performAttendanceLeaveAttachmentRequestForTest(
		t,
		service,
		employee.Email,
		created.Request.ID,
		removedAttachmentID,
	)
	assertAttendanceLeaveErrorResponse(
		t,
		removedAttachmentRecorder,
		http.StatusNotFound,
		attendanceLeaveErrorRequestNotFound,
	)
	replacementAttachmentRecorder := performAttendanceLeaveAttachmentRequestForTest(
		t,
		service,
		employee.Email,
		created.Request.ID,
		updated.Request.Attachments[0].ID,
	)
	if replacementAttachmentRecorder.Code != http.StatusOK ||
		replacementAttachmentRecorder.Body.String() != "%PDF-1.7 replacement evidence" {
		t.Fatalf(
			"replacement attachment status = %d body = %q",
			replacementAttachmentRecorder.Code,
			replacementAttachmentRecorder.Body.String(),
		)
	}

	staleRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests/"+created.Request.ID+"/update",
		employee.Email,
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-05","reason":"Stale edit","revision":1}`,
	)
	assertAttendanceLeaveErrorResponse(
		t,
		staleRecorder,
		http.StatusConflict,
		attendanceLeaveErrorInvalidStatus,
	)
	unchanged := readAttendanceLeaveDashboardForTest(t, service, employee.Email)
	if len(unchanged.Requests) != 1 ||
		unchanged.Requests[0].Revision != 2 ||
		unchanged.Requests[0].Reason != "Updated reason" {
		t.Fatalf("request after stale update = %+v", unchanged.Requests)
	}
}

func TestAttendanceLeaveRequestAllowsEmptyReasonAcrossEmployeeMutations(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-optional-leave-request-reason",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       2000,
			EffectiveOn:  "2027-01-01",
		},
		ExpiresOn: "2028-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	createRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		employee.Email,
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03"}`,
	)
	requestID := decodeAttendanceLeaveRequestIDForTest(t, createRecorder)

	updateRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests/"+requestID+"/update",
		employee.Email,
		`{"leaveTypeID":"annual","unit":"halfDay","startDate":"2027-05-04","partialPeriod":"afternoon","revision":1}`,
	)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	setAttendanceLeaveRequestStatusForTest(
		t,
		service,
		requestID,
		attendanceLeaveRequestStatusNeedsChanges,
	)

	resubmitRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests/"+requestID+"/resubmit",
		employee.Email,
		`{"leaveTypeID":"annual","unit":"halfDay","startDate":"2027-05-05","partialPeriod":"morning"}`,
	)
	if resubmitRecorder.Code != http.StatusOK {
		t.Fatalf(
			"resubmit status = %d body = %s",
			resubmitRecorder.Code,
			resubmitRecorder.Body.String(),
		)
	}

	dashboard := readAttendanceLeaveDashboardForTest(t, service, employee.Email)
	if len(dashboard.Requests) != 1 ||
		dashboard.Requests[0].Reason != "" ||
		dashboard.Requests[0].Status != attendanceLeaveRequestStatusPending ||
		dashboard.Requests[0].Revision != 3 {
		t.Fatalf("request = %+v", dashboard.Requests)
	}
}

func performAttendanceLeaveAttachmentRequestForTest(
	t *testing.T,
	service *Service,
	actorEmail string,
	requestID string,
	attachmentID string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(
		http.MethodGet,
		"/attendance/api/leave-requests/"+requestID+"/attachments/"+attachmentID,
		nil,
	)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", actorEmail)
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	return recorder
}
