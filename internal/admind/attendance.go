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
	if !service.authorizeAttendanceRequest(request) {
		http.Error(responseWriter, "attendance access required", http.StatusForbidden)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/attendance/api")
	switch {
	case request.Method == http.MethodGet && path == "/summary":
		service.writeAttendanceSummary(responseWriter, request)
	case request.Method == http.MethodPatch && path == "/settings":
		service.writeAttendanceSettings(responseWriter, request)
	case request.Method == http.MethodPost && path == "/clock":
		service.writeAttendanceClock(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) authorizeAttendanceRequest(request *http.Request) bool {
	if isLocalRequest(request) {
		return true
	}
	actorEmail := service.webStaffActorEmail(request)
	if actorEmail == "" {
		return false
	}
	return service.isFlowStaffActor(request.Context(), actorEmail)
}
