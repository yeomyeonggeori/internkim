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

type organizationCachedUsersResponse struct {
	Records         []organizationCachedUserRecord `json:"records"`
	AvailableGroups []orgGroupRecord           `json:"availableGroups,omitempty"`
}

func (service *Service) readCachedOrganizationUserList(ctx context.Context, loadSource func(context.Context) (pagesUsersResponse, error)) (pagesUsersResponse, organizationPeopleCachePolicy, error) {
	key := organizationPeopleCacheKey{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey}
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
	if errorValue != nil {
		return pagesUsersResponse{}, organizationPeopleCacheBypassed, errorValue
	}
	snapshot := snapshots[key]
	isReusableSnapshot := isReusableOrganizationPeopleCacheSnapshot(snapshot)
	previousResponse, hasPreviousResponse := cachedOrganizationUsersResponse(snapshot.PayloadJSON)
	hasPreviousResponse = isReusableSnapshot && hasPreviousResponse
	sourceRevision, errorValue := service.organizationUserSourceRevision()
	if errorValue != nil {
		return pagesUsersResponse{}, organizationPeopleCacheBypassed, errorValue
	}
	if isReusableSnapshot && snapshot.SourceRevision == sourceRevision {
		if hasPreviousResponse {
			return previousResponse, organizationPeopleCachePolicyForListRevision(snapshot.Revision, sourceRevision), nil
		}
		if errorValue := service.deleteOrganizationPeopleCacheEntries(ctx, []organizationPeopleCacheKey{key}); errorValue != nil {
			return pagesUsersResponse{}, organizationPeopleCacheBypassed, errorValue
		}
	}
	sourceResponse, errorValue := loadSource(ctx)
	if errorValue != nil {
		return pagesUsersResponse{}, organizationPeopleCacheBypassed, errorValue
	}
	response := organizationUsersResponseFromCache(newOrganizationCachedUsersResponse(sourceResponse))
	currentSourceRevision, errorValue := service.organizationUserSourceRevision()
	if errorValue != nil {
		return pagesUsersResponse{}, organizationPeopleCacheBypassed, errorValue
	}
	if currentSourceRevision != sourceRevision || snapshot.IsDirty {
		return response, organizationPeopleCacheBypassed, nil
	}
	if snapshot.SourceRevision != sourceRevision {
		if !hasPreviousResponse {
			previousResponse = pagesUsersResponse{}
		}
		if errorValue := service.invalidateChangedOrganizationUsers(ctx, previousResponse, response); errorValue != nil {
			return pagesUsersResponse{}, organizationPeopleCacheBypassed, errorValue
		}
	}
	payloadJSON, errorValue := json.Marshal(newOrganizationCachedUsersResponse(response))
	if errorValue != nil {
		return pagesUsersResponse{}, organizationPeopleCacheBypassed, fmt.Errorf("encode organization user list cache payload: %w", errorValue)
	}
	written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, key, snapshot.Revision, sourceRevision, payloadJSON)
	if errorValue != nil {
		return pagesUsersResponse{}, organizationPeopleCacheBypassed, errorValue
	}
	if !written {
		return response, organizationPeopleCacheBypassed, nil
	}
	return response, organizationPeopleCachePolicyForListRevision(snapshot.Revision, sourceRevision), nil
}

func newOrganizationCachedUsersResponse(response pagesUsersResponse) organizationCachedUsersResponse {
	records := make([]organizationCachedUserRecord, 0, len(response.Records))
	for _, record := range response.Records {
		records = append(records, newOrganizationCachedUserRecord(record))
	}
	return organizationCachedUsersResponse{
		Records:         records,
		AvailableGroups: append([]orgGroupRecord(nil), response.AvailableGroups...),
	}
}

func cachedOrganizationUsersResponse(payloadJSON []byte) (pagesUsersResponse, bool) {
	var cachedResponse organizationCachedUsersResponse
	if json.Unmarshal(payloadJSON, &cachedResponse) != nil {
		return pagesUsersResponse{}, false
	}
	if cachedResponse.Records == nil {
		return pagesUsersResponse{}, false
	}
	if !hasUniqueOrganizationCachedUserIdentities(cachedResponse.Records) {
		return pagesUsersResponse{}, false
	}
	if cachedResponse.AvailableGroups != nil && !isValidOrganizationGroups(cachedResponse.AvailableGroups) {
		return pagesUsersResponse{}, false
	}
	return organizationUsersResponseFromCache(cachedResponse), true
}

func hasUniqueOrganizationCachedUserIdentities(records []organizationCachedUserRecord) bool {
	seenUserIDs := map[string]bool{}
	seenEmails := map[string]bool{}
	for _, record := range records {
		if !isValidOrganizationCachedUserRecord(record) {
			return false
		}
		userID := strings.TrimSpace(record.UserID)
		if userID != "" {
			if seenUserIDs[userID] {
				return false
			}
			seenUserIDs[userID] = true
		}
		email := strings.ToLower(strings.TrimSpace(record.Email))
		if email != "" {
			if seenEmails[email] {
				return false
			}
			seenEmails[email] = true
		}
	}
	return true
}

func organizationUsersResponseFromCache(cachedResponse organizationCachedUsersResponse) pagesUsersResponse {
	response := pagesUsersResponse{
		Records:         make([]adminUserMutation, 0, len(cachedResponse.Records)),
		AvailableGroups: append([]orgGroupRecord(nil), cachedResponse.AvailableGroups...),
	}
	for _, cachedRecord := range cachedResponse.Records {
		response.Records = append(response.Records, applyOrganizationCachedUserRecord(adminUserMutation{}, cachedRecord))
	}
	return response
}

func (service *Service) organizationUserSourceRevision() (string, error) {
	content, errorValue := os.ReadFile(filepath.Join(service.Configuration.StateDirectory, "users-sync.json"))
	if os.IsNotExist(errorValue) {
		return "", nil
	}
	if errorValue != nil {
		return "", fmt.Errorf("read organization user source revision: %w", errorValue)
	}
	var document struct {
		Revision json.RawMessage `json:"revision"`
	}
	if errorValue := json.Unmarshal(content, &document); errorValue != nil {
		return "", fmt.Errorf("decode organization user source revision: %w", errorValue)
	}
	return strings.Trim(strings.TrimSpace(string(document.Revision)), `"`), nil
}

func (service *Service) loadOrganizationUserListSource(ctx context.Context) (pagesUsersResponse, error) {
	if service.hasDeviceAuth() {
		records, errorValue := service.currentUserRecords(ctx)
		return pagesUsersResponse{Records: records}, errorValue
	}
	response, errorValue := service.buildLocalUsersResponse(ctx)
	if errorValue != nil {
		return pagesUsersResponse{}, errorValue
	}
	for index := range response.Records {
		applyDefaultOrganizationMetadata(&response.Records[index])
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
