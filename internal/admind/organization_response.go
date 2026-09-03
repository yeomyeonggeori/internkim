package admind

import (
	"context"
	"encoding/json"
	"strings"
)

func (service *Service) withOrganizationMetadata(ctx context.Context, responseBody []byte) ([]byte, error) {
	var usersResponse pagesUsersResponse
	if errorValue := json.Unmarshal(responseBody, &usersResponse); errorValue != nil {
		return nil, errorValue
	}
	described, errorValue := service.organizationMetadataUsersResponse(ctx, usersResponse)
	if errorValue != nil {
		return nil, errorValue
	}
	return json.Marshal(described)
}

func (service *Service) organizationMetadataUsersResponse(ctx context.Context, usersResponse pagesUsersResponse) (pagesUsersResponse, error) {
	profiles, errorValue := service.organizationProfilesOfTheCompany(ctx)
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	groups, errorValue := service.organizationGroupsOfTheCompany(ctx, service.claimedAdminEmail())
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	profilesByUserID, profilesByEmail := organizationProfileIndexes(profiles)
	for index := range usersResponse.Records {
		profile, found := organizationProfileForUser(usersResponse.Records[index], profilesByUserID, profilesByEmail)
		if !found {
			continue
		}
		applyOrganizationProfile(&usersResponse.Records[index], profile)
	}
	usersResponse.AvailableGroups = groups
	return usersResponse, nil
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
