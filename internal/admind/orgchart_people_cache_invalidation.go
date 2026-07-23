package admind

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"slices"
)

func (service *Service) beginOrgchartUserMutation(ctx context.Context, identities []orgchartPersonIdentity) ([]orgchartPeopleCacheKey, error) {
	return service.beginOrgchartUserCacheMutation(ctx, orgchartUserMutationCacheKeys(identities))
}

func (service *Service) completeOrgchartUserMutation(ctx context.Context, keys []orgchartPeopleCacheKey) error {
	return service.completeOrgchartPeopleCacheMutation(ctx, keys)
}

func (service *Service) invalidateChangedOrgchartUsers(ctx context.Context, previous pagesUsersResponse, current pagesUsersResponse) error {
	identities := changedOrgchartUserIdentities(previous.Records, current.Records)
	keys := make([]orgchartPeopleCacheKey, 0, len(identities)*2)
	for _, identity := range identities {
		keys = append(keys, orgchartPersonCacheKeys(identity)...)
	}
	if len(keys) == 0 {
		return nil
	}
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return fmt.Errorf("begin changed orgchart user invalidation: %w", errorValue)
	}
	defer transaction.Rollback()
	if errorValue := invalidateOrgchartPeopleCacheKeys(ctx, transaction, keys, false); errorValue != nil {
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit changed orgchart user invalidation: %w", errorValue)
	}
	return nil
}

func changedOrgchartUserIdentities(previous []adminUserMutation, current []adminUserMutation) []orgchartPersonIdentity {
	previousByKey := orgchartUsersByIdentityKey(previous)
	currentByKey := orgchartUsersByIdentityKey(current)
	identities := []orgchartPersonIdentity{}
	for key, previousRecord := range previousByKey {
		currentRecord, found := currentByKey[key]
		if !found || !reflect.DeepEqual(previousRecord, currentRecord) {
			identities = append(identities, orgchartPersonIdentity{UserID: previousRecord.UserID, Email: previousRecord.Email})
			if found {
				identities = append(identities, orgchartPersonIdentity{UserID: currentRecord.UserID, Email: currentRecord.Email})
			}
		}
	}
	for key, currentRecord := range currentByKey {
		if _, found := previousByKey[key]; !found {
			identities = append(identities, orgchartPersonIdentity{UserID: currentRecord.UserID, Email: currentRecord.Email})
		}
	}
	return identities
}

func orgchartUsersByIdentityKey(records []adminUserMutation) map[string]adminUserMutation {
	result := make(map[string]adminUserMutation, len(records))
	for _, record := range records {
		if key, found := orgchartPersonCacheKey(record.UserID, record.Email); found {
			result[key.Key] = record
		}
	}
	return result
}

func orgchartUserMutationCacheKeys(identities []orgchartPersonIdentity) []orgchartPeopleCacheKey {
	keys := []orgchartPeopleCacheKey{{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey}}
	for _, identity := range identities {
		keys = append(keys, orgchartPersonCacheKeys(identity)...)
	}
	return uniqueOrgchartPeopleCacheKeys(keys)
}

func invalidateOrgchartProfiles(ctx context.Context, transaction *sql.Tx, profiles []orgchartProfile) error {
	keys := make([]orgchartPeopleCacheKey, 0, len(profiles)*2)
	for _, profile := range profiles {
		keys = append(keys, orgchartPersonCacheKeys(orgchartPersonIdentity{UserID: profile.UserID, Email: profile.Email})...)
	}
	return invalidateOrgchartPeopleCacheKeys(ctx, transaction, keys, false)
}

func invalidateOrgchartGroups(ctx context.Context, transaction *sql.Tx, changedGroupIDs []string) error {
	if len(changedGroupIDs) == 0 {
		return nil
	}
	identities, errorValue := orgchartProfileIdentitiesForGroups(ctx, transaction, changedGroupIDs)
	if errorValue != nil {
		return errorValue
	}
	keys := []orgchartPeopleCacheKey{{Kind: orgchartPeopleCacheGroups, Key: orgchartPeopleCacheSingletonKey}}
	for _, identity := range identities {
		keys = append(keys, orgchartPersonCacheKeys(identity)...)
	}
	return invalidateOrgchartPeopleCacheKeys(ctx, transaction, keys, false)
}

func changedOrgchartGroupIDs(previous []orgGroupRecord, next []orgGroupRecord, aliases map[string]string) []string {
	type positionedGroup struct {
		Name     string
		ParentID string
		Position int
	}
	previousByID := make(map[string]positionedGroup, len(previous))
	for index, group := range previous {
		previousByID[group.ID] = positionedGroup{Name: group.Name, ParentID: group.ParentID, Position: index}
	}
	nextByID := make(map[string]positionedGroup, len(next))
	for index, group := range next {
		nextByID[group.ID] = positionedGroup{Name: group.Name, ParentID: group.ParentID, Position: index}
	}
	changed := map[string]struct{}{}
	for groupID, group := range previousByID {
		if nextGroup, found := nextByID[groupID]; !found || nextGroup.Name != group.Name || nextGroup.ParentID != group.ParentID || nextGroup.Position != group.Position {
			changed[groupID] = struct{}{}
		}
	}
	for groupID := range nextByID {
		if _, found := previousByID[groupID]; !found {
			changed[groupID] = struct{}{}
		}
	}
	for groupID, canonicalID := range aliases {
		changed[groupID] = struct{}{}
		changed[canonicalID] = struct{}{}
	}
	result := make([]string, 0, len(changed))
	for groupID := range changed {
		result = append(result, groupID)
	}
	slices.Sort(result)
	return result
}

func orgchartProfileIdentitiesForGroups(ctx context.Context, transaction *sql.Tx, groupIDs []string) ([]orgchartPersonIdentity, error) {
	rows, errorValue := transaction.QueryContext(ctx, `SELECT user_id, email, primary_group_id, group_ids FROM orgchart_profiles`)
	if errorValue != nil {
		return nil, fmt.Errorf("read orgchart profiles for cache invalidation: %w", errorValue)
	}
	defer rows.Close()
	changedGroupIDs := make(map[string]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		changedGroupIDs[groupID] = struct{}{}
	}
	identities := []orgchartPersonIdentity{}
	for rows.Next() {
		var identity orgchartPersonIdentity
		var primaryGroupID string
		var groupIDsDocument string
		if errorValue := rows.Scan(&identity.UserID, &identity.Email, &primaryGroupID, &groupIDsDocument); errorValue != nil {
			return nil, fmt.Errorf("scan orgchart profile for cache invalidation: %w", errorValue)
		}
		profileGroupIDs := append(decodeOrgchartStringList(groupIDsDocument), primaryGroupID)
		if orgchartGroupIDsIntersect(profileGroupIDs, changedGroupIDs) {
			identities = append(identities, identity)
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, fmt.Errorf("iterate orgchart profiles for cache invalidation: %w", errorValue)
	}
	return identities, nil
}

func orgchartGroupIDsIntersect(groupIDs []string, changedGroupIDs map[string]struct{}) bool {
	for _, groupID := range groupIDs {
		if _, found := changedGroupIDs[groupID]; found {
			return true
		}
	}
	return false
}
