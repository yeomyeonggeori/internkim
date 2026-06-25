package admind

import (
	"context"
	"encoding/json"
	"strings"
)

func (service *Service) withOrgchartMetadata(ctx context.Context, responseBody []byte) ([]byte, error) {
	var usersResponse pagesUsersResponse
	if errorValue := json.Unmarshal(responseBody, &usersResponse); errorValue != nil {
		return nil, errorValue
	}
	groups, errorValue := service.readOrgchartGroupsOrInitialize(ctx, usersResponse.AvailableGroups)
	if errorValue != nil {
		return nil, errorValue
	}
	profiles, errorValue := service.readOrgchartProfiles(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	profilesByUserID, profilesByEmail := orgchartProfileIndexes(profiles)
	usersResponse.AvailableGroups = groups
	for index := range usersResponse.Records {
		applyDefaultOrgchartMetadata(&usersResponse.Records[index])
		profile, found := orgchartProfileForUser(usersResponse.Records[index], profilesByUserID, profilesByEmail)
		if !found {
			continue
		}
		applyOrgchartProfile(&usersResponse.Records[index], profile)
	}
	return json.Marshal(usersResponse)
}

func orgchartProfileIndexes(profiles []orgchartProfile) (map[string]orgchartProfile, map[string]orgchartProfile) {
	profilesByUserID := map[string]orgchartProfile{}
	profilesByEmail := map[string]orgchartProfile{}
	for _, profile := range profiles {
		if profile.UserID != "" {
			profilesByUserID[profile.UserID] = profile
		}
		if profile.Email != "" {
			profilesByEmail[profile.Email] = profile
		}
	}
	return profilesByUserID, profilesByEmail
}

func orgchartProfileForUser(record adminUserMutation, profilesByUserID map[string]orgchartProfile, profilesByEmail map[string]orgchartProfile) (orgchartProfile, bool) {
	userID := strings.TrimSpace(record.UserID)
	if userID != "" {
		if profile, found := profilesByUserID[userID]; found {
			return profile, true
		}
	}
	email := strings.ToLower(strings.TrimSpace(record.Email))
	if email != "" {
		if profile, found := profilesByEmail[email]; found {
			return profile, true
		}
	}
	return orgchartProfile{}, false
}

func applyDefaultOrgchartMetadata(record *adminUserMutation) {
	record.EmploymentStatus = orgchartEmploymentStatusActive
	record.IsOrgchartVisible = true
	if record.PrimaryGroupID == "" && record.Group != "" {
		record.PrimaryGroupID = record.Group
	}
	if len(record.GroupIDs) == 0 && record.PrimaryGroupID != "" {
		record.GroupIDs = []string{record.PrimaryGroupID}
	}
}

func applyOrgchartProfile(record *adminUserMutation, profile orgchartProfile) {
	record.UserID = firstNonEmpty(record.UserID, profile.UserID)
	record.JobTitle = profile.JobTitle
	record.Group = profile.PrimaryGroupID
	record.PositionLevel = profile.PositionLevel
	record.PrimaryGroupID = profile.PrimaryGroupID
	record.GroupIDs = profile.GroupIDs
	record.SupervisorID = profile.SupervisorID
	record.ProjectIDs = profile.ProjectIDs
	record.TeamRole = profile.TeamRole
	record.EmploymentStatus = profile.EmploymentStatus
	record.IsOrgchartVisible = profile.IsOrgchartVisible
}
