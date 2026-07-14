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

func (service *Service) readCachedOrgchartUserList(ctx context.Context, loadSource func(context.Context) (pagesUsersResponse, error)) (pagesUsersResponse, error) {
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	snapshot := snapshots[key]
	var previousResponse pagesUsersResponse
	hasPreviousResponse := snapshot.Found && snapshot.SchemaVersion == orgchartPeopleCacheSchemaVersion && json.Unmarshal(snapshot.PayloadJSON, &previousResponse) == nil
	sourceRevision, errorValue := service.orgchartUserSourceRevision()
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	if snapshot.Found && !snapshot.IsDirty && snapshot.SchemaVersion == orgchartPeopleCacheSchemaVersion && snapshot.SourceRevision == sourceRevision {
		var response pagesUsersResponse
		if json.Unmarshal(snapshot.PayloadJSON, &response) == nil {
			return response, nil
		}
		if errorValue := service.deleteOrgchartPeopleCacheEntries(ctx, []orgchartPeopleCacheKey{key}); errorValue != nil {
			return pagesUsersResponse{}, errorValue
		}
	}
	response, errorValue := loadSource(ctx)
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	currentSourceRevision, errorValue := service.orgchartUserSourceRevision()
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	if currentSourceRevision != sourceRevision || snapshot.IsDirty {
		return response, nil
	}
	if snapshot.SourceRevision != sourceRevision {
		if !hasPreviousResponse {
			previousResponse = pagesUsersResponse{}
		}
		if errorValue := service.invalidateChangedOrgchartUsers(ctx, previousResponse, response); errorValue != nil {
			return pagesUsersResponse{}, errorValue
		}
	}
	payloadJSON, errorValue := json.Marshal(sanitizedOrgchartUsersResponse(response))
	if errorValue != nil {
		return pagesUsersResponse{}, fmt.Errorf("encode orgchart user list cache payload: %w", errorValue)
	}
	if _, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, snapshot.Revision, sourceRevision, payloadJSON); errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	return response, nil
}

func sanitizedOrgchartUsersResponse(response pagesUsersResponse) pagesUsersResponse {
	result := response
	result.Records = append([]adminUserMutation(nil), response.Records...)
	for index := range result.Records {
		result.Records[index].TemporaryPassword = ""
		result.Records[index].TemporaryPasswordEmail = ""
	}
	return result
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
