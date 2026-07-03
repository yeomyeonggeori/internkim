package admind

import (
	"context"
	"encoding/json"
	"strings"
)

type orgchartMetadataResponse struct {
	response         pagesUsersResponse
	profilesByUserID map[string]orgchartProfile
	profilesByEmail  map[string]orgchartProfile
}

func (service *Service) withOrgchartMetadata(ctx context.Context, responseBody []byte) ([]byte, error) {
	metadataResponse, errorValue := service.orgchartMetadataResponse(ctx, responseBody)
	if errorValue != nil {
		return nil, errorValue
	}
	return json.Marshal(metadataResponse.response)
}

func (service *Service) orgchartMetadataResponse(ctx context.Context, responseBody []byte) (orgchartMetadataResponse, error) {
	var usersResponse pagesUsersResponse
	if errorValue := json.Unmarshal(responseBody, &usersResponse); errorValue != nil {
		return orgchartMetadataResponse{}, errorValue
	}
	groups, errorValue := service.readOrgchartGroupsOrInitialize(ctx, usersResponse.AvailableGroups)
	if errorValue != nil {
		return orgchartMetadataResponse{}, errorValue
	}
	profiles, errorValue := service.readOrgchartProfiles(ctx)
	if errorValue != nil {
		return orgchartMetadataResponse{}, errorValue
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
	return orgchartMetadataResponse{
		response:         usersResponse,
		profilesByUserID: profilesByUserID,
		profilesByEmail:  profilesByEmail,
	}, nil
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
	record.PrimaryGroupID = profile.PrimaryGroupID
	record.GroupIDs = profile.GroupIDs
	record.SupervisorID = profile.SupervisorID
}
