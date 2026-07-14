package admind

import (
	"context"
	"encoding/json"
	"strings"
)

type orgchartPeopleCachePolicy struct {
	CanUsePersonCache         bool
	HasExpectedListRevision   bool
	ExpectedListRevision      int64
	HasExpectedSourceRevision bool
	ExpectedSourceRevision    string
}

var orgchartPeopleCacheEnabled = orgchartPeopleCachePolicy{CanUsePersonCache: true}
var orgchartPeopleCacheBypassed = orgchartPeopleCachePolicy{}

func orgchartPeopleCachePolicyForListRevision(revision int64, sourceRevision string) orgchartPeopleCachePolicy {
	return orgchartPeopleCachePolicy{
		CanUsePersonCache:         true,
		HasExpectedListRevision:   true,
		ExpectedListRevision:      revision,
		HasExpectedSourceRevision: true,
		ExpectedSourceRevision:    sourceRevision,
	}
}

func (service *Service) readApplicableOrgchartPeopleCacheSnapshots(ctx context.Context, keys []orgchartPeopleCacheKey, cachePolicy orgchartPeopleCachePolicy) (map[orgchartPeopleCacheKey]orgchartPeopleCacheSnapshot, orgchartPeopleCachePolicy, error) {
	if !cachePolicy.CanUsePersonCache {
		return map[orgchartPeopleCacheKey]orgchartPeopleCacheSnapshot{}, cachePolicy, nil
	}
	if cachePolicy.HasExpectedSourceRevision {
		currentSourceRevision, errorValue := service.orgchartUserSourceRevision()
		if errorValue != nil {
			return nil, orgchartPeopleCacheBypassed, errorValue
		}
		if currentSourceRevision != cachePolicy.ExpectedSourceRevision {
			return map[orgchartPeopleCacheKey]orgchartPeopleCacheSnapshot{}, orgchartPeopleCacheBypassed, nil
		}
	}
	snapshotKeys := keys
	listKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey}
	if cachePolicy.HasExpectedListRevision {
		snapshotKeys = append(append([]orgchartPeopleCacheKey{}, keys...), listKey)
	}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, snapshotKeys)
	if errorValue != nil {
		return nil, orgchartPeopleCacheBypassed, errorValue
	}
	if cachePolicy.HasExpectedListRevision && !isExpectedOrgchartPeopleListSnapshot(snapshots[listKey], cachePolicy.ExpectedListRevision) {
		return map[orgchartPeopleCacheKey]orgchartPeopleCacheSnapshot{}, orgchartPeopleCacheBypassed, nil
	}
	return snapshots, cachePolicy, nil
}

func isExpectedOrgchartPeopleListSnapshot(snapshot orgchartPeopleCacheSnapshot, expectedRevision int64) bool {
	return snapshot.Revision == expectedRevision && !snapshot.IsDirty && snapshot.ActiveMutations == 0
}

func isReusableOrgchartPeopleCacheSnapshot(snapshot orgchartPeopleCacheSnapshot) bool {
	return snapshot.Found && !snapshot.IsDirty && snapshot.ActiveMutations == 0 && snapshot.SchemaVersion == orgchartPeopleCacheSchemaVersion
}

func cachedOrgchartPerson(snapshot orgchartPeopleCacheSnapshot, key orgchartPeopleCacheKey) (orgchartCachedPerson, bool) {
	if !isReusableOrgchartPeopleCacheSnapshot(snapshot) {
		return orgchartCachedPerson{}, false
	}
	var person orgchartCachedPerson
	if json.Unmarshal(snapshot.PayloadJSON, &person) != nil || !doesOrgchartCachedUserRecordMatchKey(person.Record, key) {
		return orgchartCachedPerson{}, false
	}
	profile, isValid := validCachedOrgchartProfile(person.Profile, person.Record)
	if !isValid {
		return orgchartCachedPerson{}, false
	}
	person.Profile = profile
	return person, true
}

func validCachedOrgchartProfile(profile *orgchartProfile, record orgchartCachedUserRecord) (*orgchartProfile, bool) {
	if profile == nil {
		return nil, true
	}
	if !isValidOrgchartEmploymentStatus(profile.EmploymentStatus) {
		return nil, false
	}
	normalizedProfile := normalizeOrgchartProfile(*profile)
	if !doesOrgchartCachedProfileMatchRecord(normalizedProfile, record) {
		return nil, false
	}
	return &normalizedProfile, true
}

func doesOrgchartCachedProfileMatchRecord(profile orgchartProfile, record orgchartCachedUserRecord) bool {
	userID := strings.TrimSpace(record.UserID)
	if userID != "" && profile.UserID == userID {
		return true
	}
	email := strings.ToLower(strings.TrimSpace(record.Email))
	return email != "" && profile.Email == email
}

func isValidOrgchartCachedUserRecord(record orgchartCachedUserRecord) bool {
	return strings.TrimSpace(record.UserID) != "" || strings.TrimSpace(record.Email) != ""
}

func doesOrgchartCachedUserRecordMatchKey(record orgchartCachedUserRecord, expectedKey orgchartPeopleCacheKey) bool {
	if !isValidOrgchartCachedUserRecord(record) {
		return false
	}
	key, found := orgchartPersonCacheKey(record.UserID, record.Email)
	return found && key == expectedKey
}

func cachedOrgchartGroups(payloadJSON []byte) ([]orgGroupRecord, bool) {
	var groups []orgGroupRecord
	if json.Unmarshal(payloadJSON, &groups) != nil || !isValidOrgchartGroups(groups) {
		return nil, false
	}
	return groups, true
}

func isValidOrgchartGroups(groups []orgGroupRecord) bool {
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
