package admind

import (
	"context"
	"encoding/json"
	"fmt"
)

func (service *Service) readCachedOrganizationGroups(ctx context.Context) ([]orgGroupRecord, error) {
	key := organizationPeopleCacheKey{Kind: organizationPeopleCacheGroups, Key: organizationPeopleCacheSingletonKey}
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
	if errorValue != nil {
		return nil, errorValue
	}
	snapshot := snapshots[key]
	if isReusableOrganizationPeopleCacheSnapshot(snapshot) {
		if groups, found := cachedOrganizationGroups(snapshot.PayloadJSON); found {
			return groups, nil
		}
		if errorValue := service.deleteOrganizationPeopleCacheEntries(ctx, []organizationPeopleCacheKey{key}); errorValue != nil {
			return nil, errorValue
		}
	}
	groups, isInitialized, errorValue := service.readOrganizationGroupsOrInitializeWithState(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	if !isInitialized {
		return groups, nil
	}
	payloadJSON, errorValue := json.Marshal(groups)
	if errorValue != nil {
		return nil, fmt.Errorf("encode organization groups cache payload: %w", errorValue)
	}
	if !snapshot.IsDirty {
		if _, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, key, snapshot.Revision, "", payloadJSON); errorValue != nil {
			return nil, errorValue
		}
	}
	return groups, nil
}
