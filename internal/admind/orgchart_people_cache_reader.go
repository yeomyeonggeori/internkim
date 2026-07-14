package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type orgchartCachedPerson struct {
	Record  orgchartCachedUserRecord `json:"record"`
	Profile *orgchartProfile         `json:"profile,omitempty"`
}

func (service *Service) applyCachedOrgchartPeople(ctx context.Context, usersResponse pagesUsersResponse) (orgchartMetadataResponse, error) {
	return service.applyOrgchartPeople(ctx, usersResponse, orgchartPeopleCacheEnabled)
}

func (service *Service) applyOrgchartPeople(ctx context.Context, usersResponse pagesUsersResponse, cachePolicy orgchartPeopleCachePolicy) (orgchartMetadataResponse, error) {
	groups, errorValue := service.readCachedOrgchartGroups(ctx, usersResponse.AvailableGroups)
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
	if cachePolicy.CanUsePersonCache && cachePolicy.HasExpectedSourceRevision {
		currentSourceRevision, errorValue := service.orgchartUserSourceRevision()
		if errorValue != nil {
			return orgchartMetadataResponse{}, errorValue
		}
		if currentSourceRevision != cachePolicy.ExpectedSourceRevision {
			cachePolicy = orgchartPeopleCacheBypassed
		}
	}
	snapshots := map[orgchartPeopleCacheKey]orgchartPeopleCacheSnapshot{}
	if cachePolicy.CanUsePersonCache {
		snapshotKeys := keys
		listKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey}
		if cachePolicy.HasExpectedListRevision {
			snapshotKeys = append(append([]orgchartPeopleCacheKey{}, keys...), listKey)
		}
		var errorValue error
		snapshots, errorValue = service.readOrgchartPeopleCacheSnapshots(ctx, snapshotKeys)
		if errorValue != nil {
			return orgchartMetadataResponse{}, errorValue
		}
		if cachePolicy.HasExpectedListRevision {
			listSnapshot := snapshots[listKey]
			if listSnapshot.Revision != cachePolicy.ExpectedListRevision || listSnapshot.IsDirty || listSnapshot.ActiveMutations != 0 {
				cachePolicy = orgchartPeopleCacheBypassed
				snapshots = map[orgchartPeopleCacheKey]orgchartPeopleCacheSnapshot{}
			}
		}
	}
	profilesByUserID := map[string]orgchartProfile{}
	profilesByEmail := map[string]orgchartProfile{}
	if orgchartPeopleCacheNeedsProfiles(usersResponse.Records, snapshots) {
		profiles, errorValue := service.readOrgchartProfiles(ctx)
		if errorValue != nil {
			return orgchartMetadataResponse{}, errorValue
		}
		profilesByUserID, profilesByEmail = orgchartProfileIndexes(profiles)
	}
	responseProfilesByUserID := map[string]orgchartProfile{}
	responseProfilesByEmail := map[string]orgchartProfile{}
	for index := range usersResponse.Records {
		record := usersResponse.Records[index]
		key, hasKey := orgchartPersonCacheKey(record.UserID, record.Email)
		if hasKey && cachePolicy.CanUsePersonCache {
			if cachedPerson, found := cachedOrgchartPerson(snapshots[key]); found {
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
			if _, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, snapshots[key].Revision, "", payloadJSON); errorValue != nil {
				return orgchartMetadataResponse{}, errorValue
			}
		}
	}
	return orgchartMetadataResponse{response: usersResponse, profilesByUserID: responseProfilesByUserID, profilesByEmail: responseProfilesByEmail}, nil
}

func orgchartPeopleCacheNeedsProfiles(records []adminUserMutation, snapshots map[orgchartPeopleCacheKey]orgchartPeopleCacheSnapshot) bool {
	for _, record := range records {
		key, found := orgchartPersonCacheKey(record.UserID, record.Email)
		if !found {
			return true
		}
		if _, found := cachedOrgchartPerson(snapshots[key]); !found {
			return true
		}
	}
	return false
}

func cachedOrgchartPerson(snapshot orgchartPeopleCacheSnapshot) (orgchartCachedPerson, bool) {
	if !snapshot.Found || snapshot.IsDirty || snapshot.SchemaVersion != orgchartPeopleCacheSchemaVersion {
		return orgchartCachedPerson{}, false
	}
	var person orgchartCachedPerson
	if json.Unmarshal(snapshot.PayloadJSON, &person) != nil {
		return orgchartCachedPerson{}, false
	}
	if !isValidOrgchartCachedUserRecord(person.Record) {
		return orgchartCachedPerson{}, false
	}
	return person, true
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

func orgchartPersonCacheKey(userID string, email string) (orgchartPeopleCacheKey, bool) {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID != "" {
		return orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: normalizedUserID}, true
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return orgchartPeopleCacheKey{}, false
	}
	return orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "email:" + normalizedEmail}, true
}
