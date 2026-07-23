package admind

import (
	"context"
	"encoding/json"
	"fmt"
)

func (service *Service) readCachedOrgchartGroups(ctx context.Context) ([]orgGroupRecord, error) {
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheGroups, Key: orgchartPeopleCacheSingletonKey}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
	if errorValue != nil {
		return nil, errorValue
	}
	snapshot := snapshots[key]
	if isReusableOrgchartPeopleCacheSnapshot(snapshot) {
		if groups, found := cachedOrgchartGroups(snapshot.PayloadJSON); found {
			return groups, nil
		}
		if errorValue := service.deleteOrgchartPeopleCacheEntries(ctx, []orgchartPeopleCacheKey{key}); errorValue != nil {
			return nil, errorValue
		}
	}
	groups, isInitialized, errorValue := service.readOrgchartGroupsOrInitializeWithState(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	if !isInitialized {
		return groups, nil
	}
	payloadJSON, errorValue := json.Marshal(groups)
	if errorValue != nil {
		return nil, fmt.Errorf("encode orgchart groups cache payload: %w", errorValue)
	}
	if !snapshot.IsDirty {
		if _, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, snapshot.Revision, "", payloadJSON); errorValue != nil {
			return nil, errorValue
		}
	}
	return groups, nil
}
