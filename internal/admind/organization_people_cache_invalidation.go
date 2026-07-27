package admind

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"slices"
)

func (service *Service) beginOrganizationUserMutation(ctx context.Context, identities []organizationPersonIdentity) ([]organizationPeopleCacheKey, error) {
	return service.beginOrganizationUserCacheMutation(ctx, organizationUserMutationCacheKeys(identities))
}

func (service *Service) completeOrganizationUserMutation(ctx context.Context, keys []organizationPeopleCacheKey) error {
	return service.completeOrganizationPeopleCacheMutation(ctx, keys)
}

func (service *Service) invalidateChangedOrganizationUsers(ctx context.Context, previous pagesUsersResponse, current pagesUsersResponse) error {
	identities := changedOrganizationUserIdentities(previous.Records, current.Records)
	keys := make([]organizationPeopleCacheKey, 0, len(identities)*2)
	for _, identity := range identities {
		keys = append(keys, organizationPersonCacheKeys(identity)...)
	}
	if len(keys) == 0 {
		return nil
	}
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return fmt.Errorf("begin changed organization user invalidation: %w", errorValue)
	}
	defer transaction.Rollback()
	if errorValue := invalidateOrganizationPeopleCacheKeys(ctx, transaction, keys, false); errorValue != nil {
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit changed organization user invalidation: %w", errorValue)
	}
	return nil
}

func changedOrganizationUserIdentities(previous []adminUserMutation, current []adminUserMutation) []organizationPersonIdentity {
	previousByKey := organizationUsersByIdentityKey(previous)
	currentByKey := organizationUsersByIdentityKey(current)
	identities := []organizationPersonIdentity{}
	for key, previousRecord := range previousByKey {
		currentRecord, found := currentByKey[key]
		if !found || !reflect.DeepEqual(previousRecord, currentRecord) {
			identities = append(identities, organizationPersonIdentity{UserID: previousRecord.UserID, Email: previousRecord.Email})
			if found {
				identities = append(identities, organizationPersonIdentity{UserID: currentRecord.UserID, Email: currentRecord.Email})
			}
		}
	}
	for key, currentRecord := range currentByKey {
		if _, found := previousByKey[key]; !found {
			identities = append(identities, organizationPersonIdentity{UserID: currentRecord.UserID, Email: currentRecord.Email})
		}
	}
	return identities
}

func organizationUsersByIdentityKey(records []adminUserMutation) map[string]adminUserMutation {
	result := make(map[string]adminUserMutation, len(records))
	for _, record := range records {
		if key, found := organizationPersonCacheKey(record.UserID, record.Email); found {
			result[key.Key] = record
		}
	}
	return result
}

func organizationUserMutationCacheKeys(identities []organizationPersonIdentity) []organizationPeopleCacheKey {
	keys := []organizationPeopleCacheKey{{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey}}
	for _, identity := range identities {
		keys = append(keys, organizationPersonCacheKeys(identity)...)
	}
	return uniqueOrganizationPeopleCacheKeys(keys)
}

func invalidateOrganizationProfiles(ctx context.Context, transaction *sql.Tx, profiles []organizationProfile) error {
	keys := make([]organizationPeopleCacheKey, 0, len(profiles)*2)
	for _, profile := range profiles {
		keys = append(keys, organizationPersonCacheKeys(organizationPersonIdentity{UserID: profile.UserID, Email: profile.Email})...)
	}
	return invalidateOrganizationPeopleCacheKeys(ctx, transaction, keys, false)
}

func invalidateOrganizationGroups(ctx context.Context, transaction *sql.Tx, changedGroupIDs []string) error {
	if len(changedGroupIDs) == 0 {
		return nil
	}
	identities, errorValue := organizationProfileIdentitiesForGroups(ctx, transaction, changedGroupIDs)
	if errorValue != nil {
		return errorValue
	}
	keys := []organizationPeopleCacheKey{{Kind: organizationPeopleCacheGroups, Key: organizationPeopleCacheSingletonKey}}
	for _, identity := range identities {
		keys = append(keys, organizationPersonCacheKeys(identity)...)
	}
	return invalidateOrganizationPeopleCacheKeys(ctx, transaction, keys, false)
}

func changedOrganizationGroupIDs(previous []orgGroupRecord, next []orgGroupRecord, aliases map[string]string) []string {
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

func organizationProfileIdentitiesForGroups(ctx context.Context, transaction *sql.Tx, groupIDs []string) ([]organizationPersonIdentity, error) {
	rows, errorValue := transaction.QueryContext(ctx, `SELECT user_id, email, primary_group_id, group_ids FROM organization_profiles`)
	if errorValue != nil {
		return nil, fmt.Errorf("read organization profiles for cache invalidation: %w", errorValue)
	}
	defer rows.Close()
	changedGroupIDs := make(map[string]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		changedGroupIDs[groupID] = struct{}{}
	}
	identities := []organizationPersonIdentity{}
	for rows.Next() {
		var identity organizationPersonIdentity
		var primaryGroupID string
		var groupIDsDocument string
		if errorValue := rows.Scan(&identity.UserID, &identity.Email, &primaryGroupID, &groupIDsDocument); errorValue != nil {
			return nil, fmt.Errorf("scan organization profile for cache invalidation: %w", errorValue)
		}
		profileGroupIDs := append(decodeOrganizationStringList(groupIDsDocument), primaryGroupID)
		if organizationGroupIDsIntersect(profileGroupIDs, changedGroupIDs) {
			identities = append(identities, identity)
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, fmt.Errorf("iterate organization profiles for cache invalidation: %w", errorValue)
	}
	return identities, nil
}

func organizationGroupIDsIntersect(groupIDs []string, changedGroupIDs map[string]struct{}) bool {
	for _, groupID := range groupIDs {
		if _, found := changedGroupIDs[groupID]; found {
			return true
		}
	}
	return false
}
