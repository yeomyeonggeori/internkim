package admind

import (
	"net/http"
	"strings"
)

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
	groups, errorValue := service.organizationGroupsOfTheCompany(request.Context(), service.recordReaderEmail(request))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, pagesUsersResponse{Records: records, AvailableGroups: groups})
}
