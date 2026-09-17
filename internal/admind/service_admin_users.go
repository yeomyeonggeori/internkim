package admind

import (
	"context"
	"fmt"
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

func (service *Service) lookupRemovableUser(ctx context.Context, targetPath string) (*adminUserMutation, error) {
	email := strings.TrimPrefix(targetPath, "/")
	if decodedEmail, errorValue := url.PathUnescape(email); errorValue == nil {
		email = decodedEmail
	}
	records, errorValue := service.companyUserRecords(ctx)
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

func (service *Service) isLastAdminDemotion(ctx context.Context, email string, role string) (bool, error) {
	if role == "admin" {
		return false, nil
	}
	records, errorValue := service.companyUserRecords(ctx)
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
