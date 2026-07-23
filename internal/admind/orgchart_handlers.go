package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

var errOrgchartProfileInvalidRequest = errors.New("invalid orgchart profile request")

type orgchartProfileInvalidRequestError string

func (errorValue orgchartProfileInvalidRequestError) Error() string {
	return string(errorValue)
}

func (errorValue orgchartProfileInvalidRequestError) Is(target error) bool {
	return target == errOrgchartProfileInvalidRequest
}

type orgchartProfileRequest struct {
	UserID         string    `json:"userID"`
	Email          string    `json:"email"`
	JobTitle       *string   `json:"jobTitle"`
	Group          *string   `json:"group"`
	PrimaryGroupID *string   `json:"primaryGroupID"`
	GroupIDs       *[]string `json:"groupIDs"`
	SupervisorID   *string   `json:"supervisorID"`
}

func (service *Service) handleOrgchartProfileUpdate(responseWriter http.ResponseWriter, request *http.Request) {
	var profilesRequest struct {
		Profiles []orgchartProfileRequest `json:"profiles"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&profilesRequest); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return
	}
	profiles, errorValue := service.orgchartProfilesFromRequest(request.Context(), profilesRequest.Profiles)
	if errorValue != nil {
		writeOrgchartProfileUpdateError(responseWriter, errorValue)
		return
	}
	if errorValue := service.writeOrgchartProfiles(request.Context(), profiles); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeFullLocalUsersResponse(responseWriter, request)
}

func writeOrgchartProfileUpdateError(responseWriter http.ResponseWriter, errorValue error) {
	if errors.Is(errorValue, errOrgchartProfileInvalidRequest) {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
}

func (service *Service) handleOrgchartGroupsUpdate(responseWriter http.ResponseWriter, request *http.Request) {
	var groupsRequest struct {
		Groups []orgGroupRecord `json:"groups"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&groupsRequest); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return
	}
	if errorValue := service.writeOrgchartGroups(request.Context(), groupsRequest.Groups); errorValue != nil {
		if errors.Is(errorValue, errOrgchartGroupInvalidHierarchy) {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeFullLocalUsersResponse(responseWriter, request)
}

func (service *Service) orgchartProfilesFromRequest(ctx context.Context, requestProfiles []orgchartProfileRequest) ([]orgchartProfile, error) {
	existingProfiles, errorValue := service.readOrgchartProfiles(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	profilesByUserID, profilesByEmail := orgchartProfileIndexes(existingProfiles)
	profiles := make([]orgchartProfile, 0, len(requestProfiles))
	for _, requestProfile := range requestProfiles {
		profile := orgchartProfileForRequest(requestProfile, profilesByUserID, profilesByEmail)
		profile = applyOrgchartProfileRequest(profile, requestProfile)
		profiles = append(profiles, profile)
	}
	if errorValue := validateOrgchartSupervisorGraph(existingProfiles, profiles); errorValue != nil {
		return nil, errorValue
	}
	return profiles, nil
}

func validateOrgchartSupervisorGraph(existingProfiles []orgchartProfile, updatedProfiles []orgchartProfile) error {
	supervisorIDByUserID := map[string]string{}
	setSupervisor := func(profile orgchartProfile) {
		normalizedProfile := normalizeOrgchartProfile(profile)
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
		if errorValue := validateOrgchartSupervisorChain(userID, supervisorIDByUserID); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func validateOrgchartSupervisorChain(userID string, supervisorIDByUserID map[string]string) error {
	visitedUserIDs := map[string]bool{}
	currentUserID := userID
	for {
		supervisorID := supervisorIDByUserID[currentUserID]
		if supervisorID == "" {
			return nil
		}
		if supervisorID == userID || visitedUserIDs[supervisorID] {
			return orgchartProfileInvalidRequestError("supervisor hierarchy cannot contain cycles")
		}
		if _, found := supervisorIDByUserID[supervisorID]; !found {
			return nil
		}
		visitedUserIDs[currentUserID] = true
		currentUserID = supervisorID
	}
}

func orgchartProfileForRequest(requestProfile orgchartProfileRequest, profilesByUserID map[string]orgchartProfile, profilesByEmail map[string]orgchartProfile) orgchartProfile {
	profile, found := orgchartProfileForUser(adminUserMutation{UserID: requestProfile.UserID, Email: requestProfile.Email}, profilesByUserID, profilesByEmail)
	if !found {
		profile = orgchartProfile{
			EmploymentStatus:  orgchartEmploymentStatusActive,
			IsOrgchartVisible: true,
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

func applyOrgchartProfileRequest(profile orgchartProfile, requestProfile orgchartProfileRequest) orgchartProfile {
	previousPrimaryGroupID := profile.PrimaryGroupID
	if requestProfile.JobTitle != nil {
		profile.JobTitle = *requestProfile.JobTitle
	}
	if requestProfile.Group != nil {
		profile.PrimaryGroupID = *requestProfile.Group
	}
	if requestProfile.PrimaryGroupID != nil {
		profile.PrimaryGroupID = *requestProfile.PrimaryGroupID
	}
	if requestProfile.GroupIDs != nil {
		profile.GroupIDs = *requestProfile.GroupIDs
	}
	if requestProfile.GroupIDs == nil && (requestProfile.Group != nil || requestProfile.PrimaryGroupID != nil) {
		profile.GroupIDs = orgchartGroupIDsReplacingPrimary(profile.GroupIDs, previousPrimaryGroupID, profile.PrimaryGroupID)
	}
	if requestProfile.SupervisorID != nil {
		profile.SupervisorID = *requestProfile.SupervisorID
	}
	return profile
}

func orgchartGroupIDsReplacingPrimary(groupIDs []string, previousPrimaryGroupID string, nextPrimaryGroupID string) []string {
	normalizedGroupIDs := normalizeOrgchartStringList(groupIDs)
	normalizedPreviousPrimaryGroupID := strings.TrimSpace(previousPrimaryGroupID)
	normalizedNextPrimaryGroupID := strings.TrimSpace(nextPrimaryGroupID)
	result := []string{}
	seenGroupIDs := map[string]bool{}
	appendGroupID := func(groupID string) {
		if groupID == "" || seenGroupIDs[groupID] {
			return
		}
		seenGroupIDs[groupID] = true
		result = append(result, groupID)
	}
	appendGroupID(normalizedNextPrimaryGroupID)
	for _, groupID := range normalizedGroupIDs {
		if groupID == normalizedPreviousPrimaryGroupID || groupID == normalizedNextPrimaryGroupID {
			continue
		}
		appendGroupID(groupID)
	}
	return result
}
