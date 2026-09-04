package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const (
	adminUserRoleAdmin  = centralplane.MemberRoleAdmin
	adminUserRoleMember = centralplane.MemberRoleMember
)

func normalizeAdminUserRole(role string) string {
	return centralplane.NormalizeMemberRole(role)
}

func (service *Service) lookupRemovableUser(ctx context.Context, fleetID string, fleetSecret string, targetPath string) (*adminUserMutation, error) {
	email := strings.TrimPrefix(targetPath, "/")
	if decodedEmail, errorValue := url.PathUnescape(email); errorValue == nil {
		email = decodedEmail
	}
	records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret)
	if errorValue != nil {
		return nil, errorValue
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	adminTotal := 0
	for index := range records {
		if records[index].Role == "admin" {
			adminTotal++
		}
	}
	for index := range records {
		record := records[index]
		if !strings.EqualFold(record.Email, normalizedEmail) {
			continue
		}
		if record.Role == "admin" && adminTotal <= 1 {
			return nil, fmt.Errorf("cannot remove the last admin user")
		}
		return &record, nil
	}
	return nil, nil
}

func (service *Service) lookupUserRecordByEmail(ctx context.Context, fleetID string, fleetSecret string, email string) (*adminUserMutation, error) {
	records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret)
	if errorValue != nil {
		return nil, errorValue
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	for index := range records {
		if strings.EqualFold(records[index].Email, normalizedEmail) {
			return &records[index], nil
		}
	}
	return nil, nil
}

func (service *Service) isLastAdminDemotion(ctx context.Context, fleetID string, fleetSecret string, email string, role string) (bool, error) {
	if role == "admin" {
		return false, nil
	}
	records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret)
	if errorValue != nil {
		return false, errorValue
	}
	adminTotal := 0
	isTargetAdmin := false
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	for _, record := range records {
		if record.Role != "admin" {
			continue
		}
		adminTotal++
		if strings.EqualFold(record.Email, normalizedEmail) {
			isTargetAdmin = true
		}
	}
	return isTargetAdmin && adminTotal <= 1, nil
}

func (service *Service) lookupUserRecords(ctx context.Context, fleetID string, fleetSecret string) ([]adminUserMutation, error) {
	requestURL := strings.TrimRight(service.Configuration.APIBaseURL, "/") + "/api/users?fleet_id=" + url.QueryEscape(fleetID)
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("X-INTERNKIM-FLEET-ID", fleetID)
	request.Header.Set("X-INTERNKIM-FLEET-SECRET", fleetSecret)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		document, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("users lookup returned %d: %s", response.StatusCode, strings.TrimSpace(string(document)))
	}
	var usersResponse pagesUsersResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&usersResponse); errorValue != nil {
		return nil, errorValue
	}
	return usersResponse.Records, nil
}
