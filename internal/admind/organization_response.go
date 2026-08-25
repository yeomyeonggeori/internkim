package admind

import (
	"context"
	"encoding/json"
	"strings"
)

type organizationMetadataResponse struct {
	response         pagesUsersResponse
	profilesByUserID map[string]organizationProfile
	profilesByEmail  map[string]organizationProfile
}

func (service *Service) withOrganizationMetadata(ctx context.Context, responseBody []byte) ([]byte, error) {
	metadataResponse, errorValue := service.organizationMetadataResponse(ctx, responseBody)
	if errorValue != nil {
		return nil, errorValue
	}
	return json.Marshal(metadataResponse.response)
}

func (service *Service) organizationMetadataResponse(ctx context.Context, responseBody []byte) (organizationMetadataResponse, error) {
	var usersResponse pagesUsersResponse
	if errorValue := json.Unmarshal(responseBody, &usersResponse); errorValue != nil {
		return organizationMetadataResponse{}, errorValue
	}
	return service.organizationMetadataUsersResponse(ctx, usersResponse)
}

func (service *Service) organizationMetadataUsersResponse(ctx context.Context, usersResponse pagesUsersResponse) (organizationMetadataResponse, error) {
	return service.applyCachedOrganizationPeople(ctx, usersResponse)
}

func organizationProfileIndexes(profiles []organizationProfile) (map[string]organizationProfile, map[string]organizationProfile) {
	profilesByUserID := map[string]organizationProfile{}
	profilesByEmail := map[string]organizationProfile{}
	for _, profile := range profiles {
		if profile.MemberID != "" {
			profilesByUserID[profile.MemberID] = profile
		}
		if profile.Email != "" {
			profilesByEmail[profile.Email] = profile
		}
	}
	return profilesByUserID, profilesByEmail
}

func organizationProfileForUser(record adminUserMutation, profilesByUserID map[string]organizationProfile, profilesByEmail map[string]organizationProfile) (organizationProfile, bool) {
	userID := strings.TrimSpace(record.MemberID)
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
	return organizationProfile{}, false
}

func applyOrganizationProfile(record *adminUserMutation, profile organizationProfile) {
	record.MemberID = firstNonEmpty(record.MemberID, profile.MemberID)
	record.JobTitle = profile.JobTitle
	record.GroupID = profile.GroupID
	record.PhoneNumber = profile.PhoneNumber
	if profile.HireDate != "" {
		record.HireDate = profile.HireDate
	}
	record.SupervisorID = profile.SupervisorID
}
