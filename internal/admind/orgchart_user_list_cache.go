package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type orgchartCachedUsersResponse struct {
	Records         []orgchartCachedUserRecord `json:"records"`
	AvailableGroups []orgGroupRecord           `json:"availableGroups,omitempty"`
}

func (service *Service) readCachedOrgchartUserList(ctx context.Context, loadSource func(context.Context) (pagesUsersResponse, error)) (pagesUsersResponse, orgchartPeopleCachePolicy, error) {
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
	if errorValue != nil {
		return pagesUsersResponse{}, orgchartPeopleCacheBypassed, errorValue
	}
	snapshot := snapshots[key]
	previousResponse, hasPreviousResponse := cachedOrgchartUsersResponse(snapshot.PayloadJSON)
	hasPreviousResponse = snapshot.Found && snapshot.SchemaVersion == orgchartPeopleCacheSchemaVersion && hasPreviousResponse
	sourceRevision, errorValue := service.orgchartUserSourceRevision()
	if errorValue != nil {
		return pagesUsersResponse{}, orgchartPeopleCacheBypassed, errorValue
	}
	if snapshot.Found && !snapshot.IsDirty && snapshot.SchemaVersion == orgchartPeopleCacheSchemaVersion && snapshot.SourceRevision == sourceRevision {
		if response, found := cachedOrgchartUsersResponse(snapshot.PayloadJSON); found {
			return response, orgchartPeopleCachePolicyForListRevision(snapshot.Revision, sourceRevision), nil
		}
		if errorValue := service.deleteOrgchartPeopleCacheEntries(ctx, []orgchartPeopleCacheKey{key}); errorValue != nil {
			return pagesUsersResponse{}, orgchartPeopleCacheBypassed, errorValue
		}
	}
	sourceResponse, errorValue := loadSource(ctx)
	if errorValue != nil {
		return pagesUsersResponse{}, orgchartPeopleCacheBypassed, errorValue
	}
	response := orgchartUsersResponseFromCache(newOrgchartCachedUsersResponse(sourceResponse))
	currentSourceRevision, errorValue := service.orgchartUserSourceRevision()
	if errorValue != nil {
		return pagesUsersResponse{}, orgchartPeopleCacheBypassed, errorValue
	}
	if currentSourceRevision != sourceRevision || snapshot.IsDirty {
		return response, orgchartPeopleCacheBypassed, nil
	}
	if snapshot.SourceRevision != sourceRevision {
		if !hasPreviousResponse {
			previousResponse = pagesUsersResponse{}
		}
		if errorValue := service.invalidateChangedOrgchartUsers(ctx, previousResponse, response); errorValue != nil {
			return pagesUsersResponse{}, orgchartPeopleCacheBypassed, errorValue
		}
	}
	payloadJSON, errorValue := json.Marshal(newOrgchartCachedUsersResponse(response))
	if errorValue != nil {
		return pagesUsersResponse{}, orgchartPeopleCacheBypassed, fmt.Errorf("encode orgchart user list cache payload: %w", errorValue)
	}
	written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, snapshot.Revision, sourceRevision, payloadJSON)
	if errorValue != nil {
		return pagesUsersResponse{}, orgchartPeopleCacheBypassed, errorValue
	}
	if !written {
		return response, orgchartPeopleCacheBypassed, nil
	}
	return response, orgchartPeopleCachePolicyForListRevision(snapshot.Revision, sourceRevision), nil
}

func newOrgchartCachedUsersResponse(response pagesUsersResponse) orgchartCachedUsersResponse {
	records := make([]orgchartCachedUserRecord, 0, len(response.Records))
	for _, record := range response.Records {
		records = append(records, newOrgchartCachedUserRecord(record))
	}
	return orgchartCachedUsersResponse{
		Records:         records,
		AvailableGroups: append([]orgGroupRecord(nil), response.AvailableGroups...),
	}
}

func cachedOrgchartUsersResponse(payloadJSON []byte) (pagesUsersResponse, bool) {
	var cachedResponse orgchartCachedUsersResponse
	if json.Unmarshal(payloadJSON, &cachedResponse) != nil {
		return pagesUsersResponse{}, false
	}
	for _, cachedRecord := range cachedResponse.Records {
		if !isValidOrgchartCachedUserRecord(cachedRecord) {
			return pagesUsersResponse{}, false
		}
	}
	return orgchartUsersResponseFromCache(cachedResponse), true
}

func orgchartUsersResponseFromCache(cachedResponse orgchartCachedUsersResponse) pagesUsersResponse {
	response := pagesUsersResponse{
		Records:         make([]adminUserMutation, 0, len(cachedResponse.Records)),
		AvailableGroups: append([]orgGroupRecord(nil), cachedResponse.AvailableGroups...),
	}
	for _, cachedRecord := range cachedResponse.Records {
		response.Records = append(response.Records, applyOrgchartCachedUserRecord(adminUserMutation{}, cachedRecord))
	}
	return response
}

func (service *Service) orgchartUserSourceRevision() (string, error) {
	content, errorValue := os.ReadFile(filepath.Join(service.Configuration.StateDirectory, "users-sync.json"))
	if os.IsNotExist(errorValue) {
		return "", nil
	}
	if errorValue != nil {
		return "", fmt.Errorf("read orgchart user source revision: %w", errorValue)
	}
	var document struct {
		Revision json.RawMessage `json:"revision"`
	}
	if errorValue := json.Unmarshal(content, &document); errorValue != nil {
		return "", fmt.Errorf("decode orgchart user source revision: %w", errorValue)
	}
	return strings.Trim(strings.TrimSpace(string(document.Revision)), `"`), nil
}

func (service *Service) loadOrgchartUserListSource(ctx context.Context) (pagesUsersResponse, error) {
	if service.hasDeviceAuth() {
		records, errorValue := service.currentUserRecords(ctx)
		return pagesUsersResponse{Records: records}, errorValue
	}
	response, errorValue := service.buildLocalUsersResponse(ctx)
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	for index := range response.Records {
		applyDefaultOrgchartMetadata(&response.Records[index])
	}
	responseBody, errorValue := json.Marshal(response)
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	enhancedBody, errorValue := service.withBlueclawCircles(ctx, responseBody)
	if errorValue != nil {
		log.Printf("Blueclaw circle merge failed: %v", errorValue)
		enhancedBody = responseBody
	}
	if errorValue := json.Unmarshal(enhancedBody, &response); errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	return response, nil
}
