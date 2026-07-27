package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

var errOrganizationProfileInvalidRequest = errors.New("invalid organization profile request")

type organizationProfileInvalidRequestError string

func (errorValue organizationProfileInvalidRequestError) Error() string {
	return string(errorValue)
}

func (errorValue organizationProfileInvalidRequestError) Is(target error) bool {
	return target == errOrganizationProfileInvalidRequest
}

type organizationProfileRequest struct {
	UserID       string  `json:"userID"`
	Email        string  `json:"email"`
	JobTitle     *string `json:"jobTitle"`
	GroupID      *string `json:"groupID"`
	PhoneNumber  *string `json:"phoneNumber"`
	HireDate     *string `json:"hireDate"`
	SupervisorID *string `json:"supervisorID"`
}

func (service *Service) handleOrganizationProfileUpdate(responseWriter http.ResponseWriter, request *http.Request) {
	var profilesRequest struct {
		Profiles []organizationProfileRequest `json:"profiles"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&profilesRequest); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return
	}
	profiles, errorValue := service.organizationProfilesFromRequest(request.Context(), profilesRequest.Profiles)
	if errorValue != nil {
		writeOrganizationProfileUpdateError(responseWriter, errorValue)
		return
	}
	if errorValue := service.writeOrganizationProfiles(request.Context(), profiles); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeFullLocalUsersResponse(responseWriter, request)
}

func writeOrganizationProfileUpdateError(responseWriter http.ResponseWriter, errorValue error) {
	if errors.Is(errorValue, errOrganizationProfileInvalidRequest) {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
}

func (service *Service) handleOrganizationGroupsUpdate(responseWriter http.ResponseWriter, request *http.Request) {
	var groupsRequest struct {
		Groups []orgGroupRecord `json:"groups"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&groupsRequest); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return
	}
	if errorValue := service.writeOrganizationGroups(request.Context(), groupsRequest.Groups); errorValue != nil {
		if errors.Is(errorValue, errOrganizationGroupInvalidHierarchy) {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeFullLocalUsersResponse(responseWriter, request)
}

func (service *Service) organizationProfilesFromRequest(ctx context.Context, requestProfiles []organizationProfileRequest) ([]organizationProfile, error) {
	existingProfiles, errorValue := service.readOrganizationProfiles(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	profilesByUserID, profilesByEmail := organizationProfileIndexes(existingProfiles)
	callingCode := service.workspaceCallingCode()
	profiles := make([]organizationProfile, 0, len(requestProfiles))
	for _, requestProfile := range requestProfiles {
		profile := organizationProfileForRequest(requestProfile, profilesByUserID, profilesByEmail)
		profile = applyOrganizationProfileRequest(profile, requestProfile)
		phoneNumber, errorValue := normalizeInternationalPhoneNumber(profile.PhoneNumber, callingCode)
		if errorValue != nil {
			return nil, errorValue
		}
		profile.PhoneNumber = phoneNumber
		profiles = append(profiles, profile)
	}
	if errorValue := validateOrganizationSupervisorGraph(existingProfiles, profiles); errorValue != nil {
		return nil, errorValue
	}
	return profiles, nil
}

func validateOrganizationSupervisorGraph(existingProfiles []organizationProfile, updatedProfiles []organizationProfile) error {
	supervisorIDByUserID := map[string]string{}
	setSupervisor := func(profile organizationProfile) {
		normalizedProfile := normalizeOrganizationProfile(profile)
		if normalizedProfile.UserID == "" {
			return
		}
		supervisorIDByUserID[normalizedProfile.UserID] = normalizedProfile.SupervisorID
	}
	for _, profile := range existingProfiles {
		setSupervisor(profile)
	}
	for _, profile := range updatedProfiles {
		setSupervisor(profile)
	}
	for userID := range supervisorIDByUserID {
		if errorValue := validateOrganizationSupervisorChain(userID, supervisorIDByUserID); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func validateOrganizationSupervisorChain(userID string, supervisorIDByUserID map[string]string) error {
	visitedUserIDs := map[string]bool{}
	currentUserID := userID
	for {
		supervisorID := supervisorIDByUserID[currentUserID]
		if supervisorID == "" {
			return nil
		}
		if supervisorID == userID || visitedUserIDs[supervisorID] {
			return organizationProfileInvalidRequestError("supervisor hierarchy cannot contain cycles")
		}
		if _, found := supervisorIDByUserID[supervisorID]; !found {
			return nil
		}
		visitedUserIDs[currentUserID] = true
		currentUserID = supervisorID
	}
}

func organizationProfileForRequest(requestProfile organizationProfileRequest, profilesByUserID map[string]organizationProfile, profilesByEmail map[string]organizationProfile) organizationProfile {
	profile, found := organizationProfileForUser(adminUserMutation{UserID: requestProfile.UserID, Email: requestProfile.Email}, profilesByUserID, profilesByEmail)
	if !found {
		profile = organizationProfile{
			EmploymentStatus:      organizationEmploymentStatusActive,
			IsOrganizationVisible: true,
		}
	}
	if strings.TrimSpace(requestProfile.UserID) != "" {
		profile.UserID = requestProfile.UserID
	}
	if strings.TrimSpace(requestProfile.Email) != "" {
		profile.Email = requestProfile.Email
	}
	return profile
}

func applyOrganizationProfileRequest(profile organizationProfile, requestProfile organizationProfileRequest) organizationProfile {
	if requestProfile.JobTitle != nil {
		profile.JobTitle = *requestProfile.JobTitle
	}
	if requestProfile.GroupID != nil {
		profile.GroupID = *requestProfile.GroupID
	}
	if requestProfile.PhoneNumber != nil {
		profile.PhoneNumber = *requestProfile.PhoneNumber
	}
	if requestProfile.HireDate != nil {
		profile.HireDate = *requestProfile.HireDate
	}
	if requestProfile.SupervisorID != nil {
		profile.SupervisorID = *requestProfile.SupervisorID
	}
	return profile
}
