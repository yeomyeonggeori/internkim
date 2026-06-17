package admind

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (service *Service) authorizeFlowRequest(request *http.Request, action string, resource string) bool {
	actorEmail := service.flowActorEmail(request)
	if actorEmail == "" {
		return false
	}
	if action == flowActionManage && resource == flowResourceDefinition {
		return service.isFlowAdminActor(request, actorEmail)
	}
	if resource == flowResourceSummary && action == flowActionRead {
		return service.isFlowStaffActor(request.Context(), actorEmail)
	}
	if resource == flowResourceTask && (action == flowActionCreate || action == flowActionUpdate || action == flowActionDelete) {
		return service.isFlowStaffActor(request.Context(), actorEmail)
	}
	return false
}

func (service *Service) flowActorEmail(request *http.Request) string {
	if actorEmail := strings.ToLower(strings.TrimSpace(request.Header.Get(flowResolvedActorHeader))); actorEmail != "" {
		return actorEmail
	}
	if actorEmail := service.webStaffActorEmail(request); actorEmail != "" {
		request.Header.Set(flowResolvedActorHeader, actorEmail)
		return actorEmail
	}
	if !isLocalRequest(request) {
		return ""
	}
	actorEmail := strings.ToLower(strings.TrimSpace(request.Header.Get(flowRequesterEmailHeader)))
	if actorEmail != "" {
		request.Header.Set(flowResolvedActorHeader, actorEmail)
	}
	return actorEmail
}

func (service *Service) webStaffActorEmail(request *http.Request) string {
	actorEmail := service.webActorEmail(request)
	if actorEmail == "" || !service.isFlowStaffActor(request.Context(), actorEmail) {
		return ""
	}
	return actorEmail
}

func (service *Service) authorizeWebStaffRequest(request *http.Request) bool {
	return service.webStaffActorEmail(request) != ""
}

func (service *Service) authorizeInternalOrWebStaffRequest(request *http.Request) bool {
	if isLocalRequest(request) {
		return true
	}
	return service.authorizeWebStaffRequest(request)
}

func (service *Service) webActorEmail(request *http.Request) string {
	if actorEmail := authenticatedCallerEmail(request); actorEmail != "" {
		return actorEmail
	}
	if actorEmail := service.mattermostSessionActorEmail(request); actorEmail != "" {
		return strings.ToLower(strings.TrimSpace(actorEmail))
	}
	return strings.ToLower(strings.TrimSpace(service.webSessionActorEmail(request)))
}

func (service *Service) mattermostSessionActorEmail(request *http.Request) string {
	cookieHeader := mattermostSessionCookieHeader(request)
	if cookieHeader == "" {
		return ""
	}
	userRecord, ok := service.mattermostSessionUser(request, cookieHeader)
	if !ok {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(userRecord.Email))
}

func mattermostSessionCookieHeader(request *http.Request) string {
	cookies := make([]string, 0, len(request.Cookies()))
	for _, cookie := range request.Cookies() {
		if strings.HasPrefix(strings.ToUpper(cookie.Name), "MM") {
			cookies = append(cookies, cookie.String())
		}
	}
	return strings.Join(cookies, "; ")
}

func (service *Service) mattermostSessionUser(request *http.Request, cookieHeader string) (mattermostUserRecord, bool) {
	if strings.TrimSpace(service.Configuration.MattermostBaseURL) == "" {
		return mattermostUserRecord{}, false
	}
	requestURL := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + "/api/v4/users/me"
	mattermostRequest, errorValue := http.NewRequestWithContext(request.Context(), http.MethodGet, requestURL, nil)
	if errorValue != nil {
		return mattermostUserRecord{}, false
	}
	mattermostRequest.Header.Set("Cookie", cookieHeader)
	response, errorValue := service.httpClient().Do(mattermostRequest)
	if errorValue != nil {
		return mattermostUserRecord{}, false
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)
		return mattermostUserRecord{}, false
	}
	var userRecord mattermostUserRecord
	if errorValue := json.NewDecoder(response.Body).Decode(&userRecord); errorValue != nil {
		return mattermostUserRecord{}, false
	}
	if strings.TrimSpace(userRecord.Email) == "" {
		return mattermostUserRecord{}, false
	}
	return userRecord, true
}

func (service *Service) isFlowStaffActor(ctx context.Context, actorEmail string) bool {
	if service.isFlowAdminEmail(ctx, actorEmail) {
		return true
	}
	if strings.TrimSpace(actorEmail) == "" {
		return false
	}
	if !service.hasDeviceAuth() {
		return true
	}
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret)
	if errorValue != nil {
		return service.isEmailInUsersSyncCache(actorEmail)
	}
	for _, record := range records {
		if strings.EqualFold(record.Email, actorEmail) && isActiveFlowUser(record) {
			return true
		}
	}
	return false
}

func (service *Service) isEmailInUsersSyncCache(actorEmail string) bool {
	normalizedEmail := strings.ToLower(strings.TrimSpace(actorEmail))
	if normalizedEmail == "" {
		return false
	}
	stateDirectory := filepath.Dir(service.Configuration.FlowDatabasePath)
	content, errorValue := os.ReadFile(filepath.Join(stateDirectory, "users-sync.json"))
	if errorValue != nil {
		return false
	}
	var cache struct {
		Users []string `json:"users"`
	}
	if json.Unmarshal(content, &cache) != nil {
		return false
	}
	for _, email := range cache.Users {
		if strings.EqualFold(strings.TrimSpace(email), normalizedEmail) {
			return true
		}
	}
	return false
}

func (service *Service) isFlowAdminActor(request *http.Request, actorEmail string) bool {
	return service.isFlowAdminEmail(request.Context(), actorEmail)
}

func (service *Service) isFlowAdminEmail(ctx context.Context, actorEmail string) bool {
	if strings.TrimSpace(actorEmail) == "" {
		return false
	}
	if service.hasDeviceAuth() {
		return service.isCurrentAdminEmail(ctx, actorEmail) || service.isClaimedAdminEmail(actorEmail)
	}
	adminEmail := service.seedAdminEmail()
	return adminEmail != "" && strings.EqualFold(actorEmail, adminEmail)
}

func isActiveFlowUser(record adminUserMutation) bool {
	status := strings.ToLower(strings.TrimSpace(record.Status))
	return status == "" || status == "active"
}
