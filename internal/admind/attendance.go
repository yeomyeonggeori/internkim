package admind

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (service *Service) serveAttendancePage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/attendance" {
		http.Redirect(responseWriter, request, "/attendance/", http.StatusFound)
		return
	}
	if service.serveAttendanceStaticFile(responseWriter, request) {
		return
	}
	service.serveAttendanceIndex(responseWriter, request)
}

func (service *Service) serveAttendanceStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/attendance/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "attendance", relativePath)
	fileInformation, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInformation.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) serveAttendanceIndex(responseWriter http.ResponseWriter, request *http.Request) {
	attendanceIndexPath := filepath.Join(service.Configuration.AdminUIPath, "attendance", "index.html")
	if fileInformation, errorValue := os.Stat(attendanceIndexPath); errorValue == nil && !fileInformation.IsDir() {
		http.ServeFile(responseWriter, request, attendanceIndexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

func (service *Service) handleAttendance(responseWriter http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/attendance/api")
	if !service.authorizeAttendanceRequest(request) {
		if isAttendanceLeaveAPIPath(path) {
			writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveAccessDenied)
			return
		}
		http.Error(responseWriter, "attendance access required", http.StatusForbidden)
		return
	}
	if isAttendanceLeaveAPIPath(path) &&
		normalizeAttendanceLeaveEmail(service.webStaffActorEmail(request)) == "" {
		writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveAccessDenied)
		return
	}
	leaveRequestCancelID, isLeaveRequestCancel := attendanceLeaveRequestActionID(path, "cancel")
	leaveRequestResubmitID, isLeaveRequestResubmit := attendanceLeaveRequestActionID(path, "resubmit")
	leaveRequestUpdateID, isLeaveRequestUpdate := attendanceLeaveRequestActionID(path, "update")
	leaveRequestAttachmentRequestID, leaveRequestAttachmentID, isLeaveRequestAttachment := attendanceLeaveRequestAttachmentPath(path)
	leaveApprovalRequestID, isLeaveApprovalRequest := attendanceLeaveApprovalRequestID(path)
	leaveManagementCancelID, isLeaveManagementCancel := attendanceLeaveRequestActionIDForPrefix(
		path,
		"/leave-management/requests/",
		"cancel",
	)
	leaveManagementTimeID, isLeaveManagementTime := attendanceLeaveRequestActionIDForPrefix(
		path,
		"/leave-management/requests/",
		"time",
	)
	switch {
	case request.Method == http.MethodGet && path == "/summary":
		service.writeAttendanceSummary(responseWriter, request)
	case request.Method == http.MethodGet && path == "/work-status":
		service.writeAttendanceWorkStatus(responseWriter, request)
	case request.Method == http.MethodGet && path == "/leave":
		service.writeAttendanceLeaveDashboard(responseWriter, request)
	case request.Method == http.MethodGet && isLeaveRequestAttachment:
		service.writeAttendanceLeaveRequestAttachment(
			responseWriter,
			request,
			leaveRequestAttachmentRequestID,
			leaveRequestAttachmentID,
		)
	case request.Method == http.MethodPatch && path == "/settings":
		service.writeAttendanceSettings(responseWriter, request)
	case request.Method == http.MethodPost && path == "/clock":
		service.writeAttendanceClock(responseWriter, request)
	case request.Method == http.MethodPost && path == "/absences":
		service.writeAttendanceAbsences(responseWriter, request)
	case request.Method == http.MethodPost && path == "/leave-requests/preview":
		service.writeAttendanceLeaveRequestPreview(responseWriter, request)
	case request.Method == http.MethodPost && path == "/leave-requests":
		service.writeAttendanceLeaveRequest(responseWriter, request)
	case request.Method == http.MethodPost && isLeaveRequestCancel:
		service.cancelAttendanceLeaveRequestResponse(responseWriter, request, leaveRequestCancelID)
	case request.Method == http.MethodPost && isLeaveRequestResubmit:
		service.resubmitAttendanceLeaveRequestResponse(responseWriter, request, leaveRequestResubmitID)
	case request.Method == http.MethodPost && isLeaveRequestUpdate:
		service.updateAttendanceLeaveRequestResponse(responseWriter, request, leaveRequestUpdateID)
	case request.Method == http.MethodGet && path == "/leave-approvals":
		service.writeAttendanceLeaveApprovalInbox(responseWriter, request)
	case request.Method == http.MethodPost && isLeaveApprovalRequest:
		service.writeAttendanceLeaveApprovalDecision(responseWriter, request, leaveApprovalRequestID)
	case request.Method == http.MethodGet && path == "/leave-management":
		service.writeAttendanceLeaveManagement(responseWriter, request)
	case request.Method == http.MethodGet && path == "/leave-management/legacy-migration":
		service.writeAttendanceLegacyAbsenceMigrationPreview(responseWriter, request)
	case request.Method == http.MethodPost && path == "/leave-management/legacy-migration/apply":
		service.writeAttendanceLegacyAbsenceMigrationApply(responseWriter, request)
	case request.Method == http.MethodPost && path == "/leave-management/legacy-migration/rollback":
		service.writeAttendanceLegacyAbsenceMigrationRollback(responseWriter, request)
	case request.Method == http.MethodPost && path == "/leave-management/adjustments":
		service.writeAttendanceLeaveManagementAdjustment(responseWriter, request)
	case request.Method == http.MethodPost && path == "/leave-management/past-leaves":
		service.writeAttendanceLeaveManagementPastLeave(responseWriter, request)
	case request.Method == http.MethodPost && isLeaveManagementCancel:
		service.writeAttendanceLeaveManagementCancellation(
			responseWriter,
			request,
			leaveManagementCancelID,
		)
	case request.Method == http.MethodPost && isLeaveManagementTime:
		service.writeAttendanceLeaveManagementTimeCorrection(
			responseWriter,
			request,
			leaveManagementTimeID,
		)
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/absences/"):
		service.deleteAttendanceAbsence(responseWriter, request, strings.TrimPrefix(path, "/absences/"))
	case request.Method == http.MethodPatch && strings.HasPrefix(path, "/events/"):
		service.writeAttendanceEventOverride(responseWriter, request, strings.TrimPrefix(path, "/events/"))
	default:
		if isAttendanceLeaveAPIPath(path) {
			if allowedMethod, found := attendanceLeaveAPIAllowedMethod(
				path,
				isLeaveRequestCancel,
				isLeaveRequestResubmit,
				isLeaveRequestUpdate,
				isLeaveRequestAttachment,
				isLeaveApprovalRequest,
				isLeaveManagementCancel,
				isLeaveManagementTime,
			); found {
				responseWriter.Header().Set("Allow", allowedMethod)
				writeAttendanceLeaveRequestError(
					responseWriter,
					errAttendanceLeaveMethodNotAllowed,
				)
				return
			}
			writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveRequestNotFound)
			return
		}
		http.NotFound(responseWriter, request)
	}
}

func attendanceLeaveAPIAllowedMethod(
	path string,
	isCancel bool,
	isResubmit bool,
	isUpdate bool,
	isAttachment bool,
	isApprovalRequest bool,
	isLeaveManagementCancel bool,
	isLeaveManagementTime bool,
) (string, bool) {
	switch {
	case path == "/leave",
		isAttachment,
		path == "/leave-approvals",
		path == "/leave-management",
		path == "/leave-management/legacy-migration":
		return http.MethodGet, true
	case path == "/leave-requests",
		path == "/leave-requests/preview",
		isCancel,
		isResubmit,
		isUpdate,
		isApprovalRequest,
		path == "/leave-management/legacy-migration/apply",
		path == "/leave-management/legacy-migration/rollback",
		path == "/leave-management/adjustments",
		path == "/leave-management/past-leaves",
		isLeaveManagementCancel,
		isLeaveManagementTime:
		return http.MethodPost, true
	default:
		return "", false
	}
}

func attendanceLeaveRequestActionIDForPrefix(
	path string,
	prefix string,
	action string,
) (string, bool) {
	suffix := "/" + action
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	requestID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if requestID == "" || strings.Contains(requestID, "/") {
		return "", false
	}
	return requestID, true
}

func isAttendanceLeaveAPIPath(path string) bool {
	return path == "/leave" ||
		strings.HasPrefix(path, "/leave/") ||
		path == "/leave-requests" ||
		strings.HasPrefix(path, "/leave-requests/") ||
		path == "/leave-approvals" ||
		strings.HasPrefix(path, "/leave-approvals/") ||
		path == "/leave-management" ||
		strings.HasPrefix(path, "/leave-management/")
}

func (service *Service) authorizeAttendanceRequest(request *http.Request) bool {
	return service.authorizeInternalOrWebStaffRequest(request)
}
