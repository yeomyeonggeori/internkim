package admind

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (service *Service) serveOrganizationPage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/organization" {
		http.Redirect(responseWriter, request, "/organization/", http.StatusFound)
		return
	}
	if service.serveOrganizationStaticFile(responseWriter, request) {
		return
	}
	service.serveOrganizationIndex(responseWriter, request)
}

func (service *Service) serveOrganizationStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/organization/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "organization", relativePath)
	fileInformation, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInformation.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) serveOrganizationIndex(responseWriter http.ResponseWriter, request *http.Request) {
	organizationIndexPath := filepath.Join(service.Configuration.AdminUIPath, "organization", "index.html")
	if fileInformation, errorValue := os.Stat(organizationIndexPath); errorValue == nil && !fileInformation.IsDir() {
		http.ServeFile(responseWriter, request, organizationIndexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

func (service *Service) handleOrganization(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebMemberRequest(request) {
		http.Error(responseWriter, "organization access required", http.StatusForbidden)
		return
	}
	if request.Method != http.MethodGet || strings.TrimPrefix(request.URL.Path, "/organization/api") != "/people" {
		http.NotFound(responseWriter, request)
		return
	}
	records, errorValue := service.organizationRecordsOfTheCompany(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	groups, errorValue := service.organizationGroupsOfTheCompany(request.Context(), service.organizationReaderEmail(request))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, pagesUsersResponse{Records: records, AvailableGroups: groups})
}
