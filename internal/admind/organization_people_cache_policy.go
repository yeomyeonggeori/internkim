package admind

import (
	"context"
	"encoding/json"
	"strings"
)

type organizationPeopleCachePolicy struct {
	CanUsePersonCache         bool
	HasExpectedListRevision   bool
	ExpectedListRevision      int64
	HasExpectedSourceRevision bool
	ExpectedSourceRevision    string
}

var organizationPeopleCacheEnabled = organizationPeopleCachePolicy{CanUsePersonCache: true}
var organizationPeopleCacheBypassed = organizationPeopleCachePolicy{}

func organizationPeopleCachePolicyForListRevision(revision int64, sourceRevision string) organizationPeopleCachePolicy {
	return organizationPeopleCachePolicy{
		CanUsePersonCache:         true,
		HasExpectedListRevision:   true,
		ExpectedListRevision:      revision,
		HasExpectedSourceRevision: true,
		ExpectedSourceRevision:    sourceRevision,
	}
}

func (service *Service) readApplicableOrganizationPeopleCacheSnapshots(ctx context.Context, keys []organizationPeopleCacheKey, cachePolicy organizationPeopleCachePolicy) (map[organizationPeopleCacheKey]organizationPeopleCacheSnapshot, organizationPeopleCachePolicy, error) {
	if !cachePolicy.CanUsePersonCache {
		return map[organizationPeopleCacheKey]organizationPeopleCacheSnapshot{}, cachePolicy, nil
	}
	if cachePolicy.HasExpectedSourceRevision {
		currentSourceRevision, errorValue := service.organizationUserSourceRevision()
		if errorValue != nil {
			return nil, organizationPeopleCacheBypassed, errorValue
		}
		if currentSourceRevision != cachePolicy.ExpectedSourceRevision {
			return map[organizationPeopleCacheKey]organizationPeopleCacheSnapshot{}, organizationPeopleCacheBypassed, nil
		}
	}
	listKey := organizationPeopleCacheKey{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey}
	snapshotKeys := append(append([]organizationPeopleCacheKey{}, keys...), listKey)
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, snapshotKeys)
	if errorValue != nil {
		return nil, organizationPeopleCacheBypassed, errorValue
	}
	listSnapshot := snapshots[listKey]
	if listSnapshot.IsDirty || listSnapshot.ActiveMutations != 0 {
		return map[organizationPeopleCacheKey]organizationPeopleCacheSnapshot{}, organizationPeopleCacheBypassed, nil
	}
	if cachePolicy.HasExpectedListRevision && listSnapshot.Revision != cachePolicy.ExpectedListRevision {
		return map[organizationPeopleCacheKey]organizationPeopleCacheSnapshot{}, organizationPeopleCacheBypassed, nil
	}
	cachePolicy.HasExpectedListRevision = true
	cachePolicy.ExpectedListRevision = listSnapshot.Revision
	return snapshots, cachePolicy, nil
}

func isReusableOrganizationPeopleCacheSnapshot(snapshot organizationPeopleCacheSnapshot) bool {
	return snapshot.Found && !snapshot.IsDirty && snapshot.ActiveMutations == 0 && snapshot.SchemaVersion == organizationPeopleCacheSchemaVersion
}

func cachedOrganizationPerson(snapshot organizationPeopleCacheSnapshot, key organizationPeopleCacheKey) (organizationCachedPerson, bool) {
	if !isReusableOrganizationPeopleCacheSnapshot(snapshot) {
		return organizationCachedPerson{}, false
	}
	var person organizationCachedPerson
	if json.Unmarshal(snapshot.PayloadJSON, &person) != nil || !doesOrganizationCachedUserRecordMatchKey(person.Record, key) {
		return organizationCachedPerson{}, false
	}
	profile, isValid := validCachedOrganizationProfile(person.Profile, person.Record)
	if !isValid {
		return organizationCachedPerson{}, false
	}
	person.Profile = profile
	return person, true
}

func validCachedOrganizationProfile(profile *organizationProfile, record organizationCachedUserRecord) (*organizationProfile, bool) {
	if profile == nil {
		return nil, true
	}
	if !isValidOrganizationEmploymentStatus(profile.EmploymentStatus) {
		return nil, false
	}
	normalizedProfile := normalizeOrganizationProfile(*profile)
	if !doesOrganizationCachedProfileMatchRecord(normalizedProfile, record) {
		return nil, false
	}
	return &normalizedProfile, true
}

func doesOrganizationCachedProfileMatchRecord(profile organizationProfile, record organizationCachedUserRecord) bool {
	userID := strings.TrimSpace(record.UserID)
	if userID != "" && profile.UserID == userID {
		return true
	}
	email := strings.ToLower(strings.TrimSpace(record.Email))
	return email != "" && profile.Email == email
}

func isValidOrganizationCachedUserRecord(record organizationCachedUserRecord) bool {
	return strings.TrimSpace(record.UserID) != "" || strings.TrimSpace(record.Email) != ""
}

func doesOrganizationCachedUserRecordMatchKey(record organizationCachedUserRecord, expectedKey organizationPeopleCacheKey) bool {
	if !isValidOrganizationCachedUserRecord(record) {
		return false
	}
	key, found := organizationPersonCacheKey(record.UserID, record.Email)
	return found && key == expectedKey
}

func cachedOrganizationGroups(payloadJSON []byte) ([]orgGroupRecord, bool) {
	var groups []orgGroupRecord
	if json.Unmarshal(payloadJSON, &groups) != nil || !isValidOrganizationGroups(groups) {
		return nil, false
	}
	return groups, true
}

func isValidOrganizationGroups(groups []orgGroupRecord) bool {
	if groups == nil {
		return false
	}
	seenIDs := map[string]bool{}
	seenNames := map[string]bool{}
	for _, group := range groups {
		id := strings.TrimSpace(group.ID)
		name := strings.TrimSpace(group.Name)
		nameKey := strings.ToLower(name)
		if id == "" || name == "" || id != group.ID || name != group.Name || seenIDs[id] || seenNames[nameKey] {
			return false
		}
		seenIDs[id] = true
		seenNames[nameKey] = true
	}
	return true
}
