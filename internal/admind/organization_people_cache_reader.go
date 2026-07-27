package admind

import (
	"context"
	"encoding/json"
	"fmt"
)

type organizationCachedPerson struct {
	Record  organizationCachedUserRecord `json:"record"`
	Profile *organizationProfile         `json:"profile,omitempty"`
}

func (service *Service) applyCachedOrganizationPeople(ctx context.Context, usersResponse pagesUsersResponse) (organizationMetadataResponse, error) {
	return service.applyOrganizationPeople(ctx, usersResponse, organizationPeopleCacheEnabled)
}

func (service *Service) applyOrganizationPeople(ctx context.Context, usersResponse pagesUsersResponse, cachePolicy organizationPeopleCachePolicy) (organizationMetadataResponse, error) {
	groups, errorValue := service.readCachedOrganizationGroups(ctx)
	if errorValue != nil {
		return organizationMetadataResponse{}, errorValue
	}
	usersResponse.AvailableGroups = groups
	keys := make([]organizationPeopleCacheKey, 0, len(usersResponse.Records))
	for _, record := range usersResponse.Records {
		if key, found := organizationPersonCacheKey(record.UserID, record.Email); found {
			keys = append(keys, key)
		}
	}
	snapshots, cachePolicy, errorValue := service.readApplicableOrganizationPeopleCacheSnapshots(ctx, keys, cachePolicy)
	if errorValue != nil {
		return organizationMetadataResponse{}, errorValue
	}
	cachedPeopleByKey, needsProfiles := cachedOrganizationPeopleForRecords(usersResponse.Records, snapshots)
	profilesByUserID := map[string]organizationProfile{}
	profilesByEmail := map[string]organizationProfile{}
	if needsProfiles {
		profiles, errorValue := service.readOrganizationProfiles(ctx)
		if errorValue != nil {
			return organizationMetadataResponse{}, errorValue
		}
		profilesByUserID, profilesByEmail = organizationProfileIndexes(profiles)
	}
	responseProfilesByUserID := map[string]organizationProfile{}
	responseProfilesByEmail := map[string]organizationProfile{}
	writes := make([]organizationPeopleCacheWrite, 0, len(usersResponse.Records))
	for index := range usersResponse.Records {
		record := usersResponse.Records[index]
		key, hasKey := organizationPersonCacheKey(record.UserID, record.Email)
		if hasKey && cachePolicy.CanUsePersonCache {
			if cachedPerson, found := cachedPeopleByKey[key]; found {
				usersResponse.Records[index] = applyOrganizationCachedUserRecord(record, cachedPerson.Record)
				indexCachedOrganizationProfile(cachedPerson.Profile, responseProfilesByUserID, responseProfilesByEmail)
				continue
			}
			if snapshots[key].Found {
				if errorValue := service.deleteOrganizationPeopleCacheEntries(ctx, []organizationPeopleCacheKey{key}); errorValue != nil {
					return organizationMetadataResponse{}, errorValue
				}
			}
		}
		profile, found := organizationProfileForUser(record, profilesByUserID, profilesByEmail)
		var cachedProfile *organizationProfile
		if found {
			applyOrganizationProfile(&record, profile)
			cachedProfile = &profile
			indexCachedOrganizationProfile(cachedProfile, responseProfilesByUserID, responseProfilesByEmail)
		}
		record.TemporaryPassword = ""
		record.TemporaryPasswordEmail = ""
		usersResponse.Records[index] = record
		if hasKey && cachePolicy.CanUsePersonCache && !snapshots[key].IsDirty {
			payloadJSON, errorValue := json.Marshal(organizationCachedPerson{Record: newOrganizationCachedUserRecord(record), Profile: cachedProfile})
			if errorValue != nil {
				return organizationMetadataResponse{}, fmt.Errorf("encode organization person cache payload: %w", errorValue)
			}
			writes = append(writes, organizationPeopleCacheWrite{
				Key:                     key,
				Revision:                snapshots[key].Revision,
				HasExpectedListRevision: cachePolicy.HasExpectedListRevision,
				ExpectedListRevision:    cachePolicy.ExpectedListRevision,
				PayloadJSON:             payloadJSON,
			})
		}
	}
	if _, errorValue := service.writeOrganizationPeopleCachePayloadsIfCurrent(ctx, writes); errorValue != nil {
		return organizationMetadataResponse{}, errorValue
	}
	return organizationMetadataResponse{response: usersResponse, profilesByUserID: responseProfilesByUserID, profilesByEmail: responseProfilesByEmail}, nil
}

func cachedOrganizationPeopleForRecords(records []adminUserMutation, snapshots map[organizationPeopleCacheKey]organizationPeopleCacheSnapshot) (map[organizationPeopleCacheKey]organizationCachedPerson, bool) {
	cachedPeopleByKey := make(map[organizationPeopleCacheKey]organizationCachedPerson, len(records))
	needsProfiles := false
	for _, record := range records {
		key, found := organizationPersonCacheKey(record.UserID, record.Email)
		if !found {
			needsProfiles = true
			continue
		}
		cachedPerson, found := cachedOrganizationPerson(snapshots[key], key)
		if !found {
			needsProfiles = true
			continue
		}
		cachedPeopleByKey[key] = cachedPerson
	}
	return cachedPeopleByKey, needsProfiles
}

func indexCachedOrganizationProfile(profile *organizationProfile, profilesByUserID map[string]organizationProfile, profilesByEmail map[string]organizationProfile) {
	if profile == nil {
		return
	}
	if profile.UserID != "" {
		profilesByUserID[profile.UserID] = *profile
	}
	if profile.Email != "" {
		profilesByEmail[profile.Email] = *profile
	}
}
