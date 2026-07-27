package admind

import (
	"encoding/json"
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
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "organization access required", http.StatusForbidden)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/organization/api")
	switch {
	case request.Method == http.MethodGet && path == "/people":
		service.writeOrganizationDirectory(responseWriter, request)
	case request.Method == http.MethodPut && path == "/me/phone-number":
		service.updateOwnPhoneNumber(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) writeOrganizationDirectory(responseWriter http.ResponseWriter, request *http.Request) {
	response, errorValue := service.buildOrganizationDirectoryResponse(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) buildOrganizationDirectoryResponse(request *http.Request) (organizationDirectoryResponse, error) {
	metadataResponse, errorValue := service.organizationUsersMetadataResponse(request)
	if errorValue != nil {
		return organizationDirectoryResponse{}, errorValue
	}
	visibleResponse := visibleOrganizationUsersResponse(metadataResponse.response, metadataResponse.profilesByUserID, metadataResponse.profilesByEmail)
	return newOrganizationDirectoryResponse(visibleResponse), nil
}

func (service *Service) organizationUsersMetadataResponse(request *http.Request) (organizationMetadataResponse, error) {
	response, cachePolicy, errorValue := service.readCachedOrganizationUserList(request.Context(), service.loadOrganizationUserListSource)
	if errorValue != nil {
		return organizationMetadataResponse{}, errorValue
	}
	return service.applyOrganizationPeople(request.Context(), response, cachePolicy)
}

func visibleOrganizationUsersResponse(response pagesUsersResponse, profilesByUserID map[string]organizationProfile, profilesByEmail map[string]organizationProfile) pagesUsersResponse {
	visibleRecords := make([]adminUserMutation, 0, len(response.Records))
	visibleGroupIDs := map[string]bool{}
	for _, record := range response.Records {
		if !isVisibleOrganizationRecord(record, profilesByUserID, profilesByEmail) {
			continue
		}
		visibleRecords = append(visibleRecords, record)
		if strings.TrimSpace(record.GroupID) != "" {
			visibleGroupIDs[record.GroupID] = true
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

func isVisibleOrganizationRecord(record adminUserMutation, profilesByUserID map[string]organizationProfile, profilesByEmail map[string]organizationProfile) bool {
	profile, found := organizationProfileForUser(record, profilesByUserID, profilesByEmail)
	if !found {
		return true
	}
	return profile.IsOrganizationVisible && profile.EmploymentStatus != organizationEmploymentStatusResigned
}

func (service *Service) updateOwnPhoneNumber(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := service.webActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "organization access required", http.StatusForbidden)
		return
	}
	var payload struct {
		PhoneNumber string `json:"phoneNumber"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return
	}
	phoneNumber, errorValue := normalizeInternationalPhoneNumber(payload.PhoneNumber, service.workspaceCallingCode())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	profiles, errorValue := service.readOrganizationProfiles(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	_, profilesByEmail := organizationProfileIndexes(profiles)
	profile := profilesByEmail[actorEmail]
	profile.Email = actorEmail
	profile.PhoneNumber = phoneNumber
	if profile.EmploymentStatus == "" {
		profile.EmploymentStatus = organizationEmploymentStatusActive
		profile.IsOrganizationVisible = true
	}
	if errorValue := service.writeOrganizationProfiles(request.Context(), []organizationProfile{profile}); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]string{"phoneNumber": phoneNumber})
}
