package admind

import (
	"context"
	"encoding/json"
	"fmt"
)

type orgchartCachedPerson struct {
	Record  orgchartCachedUserRecord `json:"record"`
	Profile *orgchartProfile         `json:"profile,omitempty"`
}

func (service *Service) applyCachedOrgchartPeople(ctx context.Context, usersResponse pagesUsersResponse) (orgchartMetadataResponse, error) {
	return service.applyOrgchartPeople(ctx, usersResponse, orgchartPeopleCacheEnabled)
}

func (service *Service) applyOrgchartPeople(ctx context.Context, usersResponse pagesUsersResponse, cachePolicy orgchartPeopleCachePolicy) (orgchartMetadataResponse, error) {
	groups, errorValue := service.readCachedOrgchartGroups(ctx)
	if errorValue != nil {
		return orgchartMetadataResponse{}, errorValue
	}
	usersResponse.AvailableGroups = groups
	keys := make([]orgchartPeopleCacheKey, 0, len(usersResponse.Records))
	for _, record := range usersResponse.Records {
		if key, found := orgchartPersonCacheKey(record.UserID, record.Email); found {
			keys = append(keys, key)
		}
	}
	snapshots, cachePolicy, errorValue := service.readApplicableOrgchartPeopleCacheSnapshots(ctx, keys, cachePolicy)
	if errorValue != nil {
		return orgchartMetadataResponse{}, errorValue
	}
	cachedPeopleByKey, needsProfiles := cachedOrgchartPeopleForRecords(usersResponse.Records, snapshots)
	profilesByUserID := map[string]orgchartProfile{}
	profilesByEmail := map[string]orgchartProfile{}
	if needsProfiles {
		profiles, errorValue := service.readOrgchartProfiles(ctx)
		if errorValue != nil {
			return orgchartMetadataResponse{}, errorValue
		}
		profilesByUserID, profilesByEmail = orgchartProfileIndexes(profiles)
	}
	responseProfilesByUserID := map[string]orgchartProfile{}
	responseProfilesByEmail := map[string]orgchartProfile{}
	writes := make([]orgchartPeopleCacheWrite, 0, len(usersResponse.Records))
	for index := range usersResponse.Records {
		record := usersResponse.Records[index]
		key, hasKey := orgchartPersonCacheKey(record.UserID, record.Email)
		if hasKey && cachePolicy.CanUsePersonCache {
			if cachedPerson, found := cachedPeopleByKey[key]; found {
				usersResponse.Records[index] = applyOrgchartCachedUserRecord(record, cachedPerson.Record)
				indexCachedOrgchartProfile(cachedPerson.Profile, responseProfilesByUserID, responseProfilesByEmail)
				continue
			}
			if snapshots[key].Found {
				if errorValue := service.deleteOrgchartPeopleCacheEntries(ctx, []orgchartPeopleCacheKey{key}); errorValue != nil {
					return orgchartMetadataResponse{}, errorValue
				}
			}
		}
		applyDefaultOrgchartMetadata(&record)
		profile, found := orgchartProfileForUser(record, profilesByUserID, profilesByEmail)
		var cachedProfile *orgchartProfile
		if found {
			applyOrgchartProfile(&record, profile)
			cachedProfile = &profile
			indexCachedOrgchartProfile(cachedProfile, responseProfilesByUserID, responseProfilesByEmail)
		}
		record.TemporaryPassword = ""
		record.TemporaryPasswordEmail = ""
		usersResponse.Records[index] = record
		if hasKey && cachePolicy.CanUsePersonCache && !snapshots[key].IsDirty {
			payloadJSON, errorValue := json.Marshal(orgchartCachedPerson{Record: newOrgchartCachedUserRecord(record), Profile: cachedProfile})
			if errorValue != nil {
				return orgchartMetadataResponse{}, fmt.Errorf("encode orgchart person cache payload: %w", errorValue)
			}
			writes = append(writes, orgchartPeopleCacheWrite{
				Key:                     key,
				Revision:                snapshots[key].Revision,
				HasExpectedListRevision: cachePolicy.HasExpectedListRevision,
				ExpectedListRevision:    cachePolicy.ExpectedListRevision,
				PayloadJSON:             payloadJSON,
			})
		}
	}
	if _, errorValue := service.writeOrgchartPeopleCachePayloadsIfCurrent(ctx, writes); errorValue != nil {
		return orgchartMetadataResponse{}, errorValue
	}
	return orgchartMetadataResponse{response: usersResponse, profilesByUserID: responseProfilesByUserID, profilesByEmail: responseProfilesByEmail}, nil
}

func cachedOrgchartPeopleForRecords(records []adminUserMutation, snapshots map[orgchartPeopleCacheKey]orgchartPeopleCacheSnapshot) (map[orgchartPeopleCacheKey]orgchartCachedPerson, bool) {
	cachedPeopleByKey := make(map[orgchartPeopleCacheKey]orgchartCachedPerson, len(records))
	needsProfiles := false
	for _, record := range records {
		key, found := orgchartPersonCacheKey(record.UserID, record.Email)
		if !found {
			needsProfiles = true
			continue
		}
		cachedPerson, found := cachedOrgchartPerson(snapshots[key], key)
		if !found {
			needsProfiles = true
			continue
		}
		cachedPeopleByKey[key] = cachedPerson
	}
	return cachedPeopleByKey, needsProfiles
}

func indexCachedOrgchartProfile(profile *orgchartProfile, profilesByUserID map[string]orgchartProfile, profilesByEmail map[string]orgchartProfile) {
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
