package admind

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (service *Service) serveOrgchartPage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/orgchart" {
		http.Redirect(responseWriter, request, "/orgchart/", http.StatusFound)
		return
	}
	if service.serveOrgchartStaticFile(responseWriter, request) {
		return
	}
	service.serveOrgchartIndex(responseWriter, request)
}

func (service *Service) serveOrgchartStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/orgchart/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "orgchart", relativePath)
	fileInformation, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInformation.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) serveOrgchartIndex(responseWriter http.ResponseWriter, request *http.Request) {
	orgchartIndexPath := filepath.Join(service.Configuration.AdminUIPath, "orgchart", "index.html")
	if fileInformation, errorValue := os.Stat(orgchartIndexPath); errorValue == nil && !fileInformation.IsDir() {
		http.ServeFile(responseWriter, request, orgchartIndexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

func (service *Service) handleOrgchart(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "orgchart access required", http.StatusForbidden)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/orgchart/api")
	switch {
	case request.Method == http.MethodGet && path == "/people":
		service.writeOrgchartDirectory(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) writeOrgchartDirectory(responseWriter http.ResponseWriter, request *http.Request) {
	response, errorValue := service.buildOrgchartDirectoryResponse(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) buildOrgchartDirectoryResponse(request *http.Request) (pagesUsersResponse, error) {
	metadataResponse, errorValue := service.orgchartUsersMetadataResponse(request)
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	return visibleOrgchartUsersResponse(metadataResponse.response, metadataResponse.profilesByUserID, metadataResponse.profilesByEmail), nil
}

func (service *Service) orgchartUsersMetadataResponse(request *http.Request) (orgchartMetadataResponse, error) {
	response, errorValue := service.readCachedOrgchartUserList(request.Context(), service.loadOrgchartUserListSource)
	if errorValue != nil {
		return orgchartMetadataResponse{}, errorValue
	}
	return service.orgchartMetadataUsersResponse(request.Context(), response)
}

func visibleOrgchartUsersResponse(response pagesUsersResponse, profilesByUserID map[string]orgchartProfile, profilesByEmail map[string]orgchartProfile) pagesUsersResponse {
	visibleRecords := make([]adminUserMutation, 0, len(response.Records))
	visibleGroupIDs := map[string]bool{}
	for _, record := range response.Records {
		if !isVisibleOrgchartRecord(record, profilesByUserID, profilesByEmail) {
			continue
		}
		visibleRecords = append(visibleRecords, record)
		for _, groupID := range record.GroupIDs {
			if strings.TrimSpace(groupID) != "" {
				visibleGroupIDs[groupID] = true
			}
		}
		if strings.TrimSpace(record.PrimaryGroupID) != "" {
			visibleGroupIDs[record.PrimaryGroupID] = true
		}
	}
	visibleGroups := make([]orgGroupRecord, 0, len(response.AvailableGroups))
	for _, group := range response.AvailableGroups {
		if visibleGroupIDs[group.ID] {
			visibleGroups = append(visibleGroups, group)
		}
	}
	response.Records = visibleRecords
	response.AvailableGroups = visibleGroups
	response.AvailableCircles = nil
	return response
}

func isVisibleOrgchartRecord(record adminUserMutation, profilesByUserID map[string]orgchartProfile, profilesByEmail map[string]orgchartProfile) bool {
	profile, found := orgchartProfileForUser(record, profilesByUserID, profilesByEmail)
	if !found {
		return true
	}
	return profile.IsOrgchartVisible && profile.EmploymentStatus != orgchartEmploymentStatusResigned
}
