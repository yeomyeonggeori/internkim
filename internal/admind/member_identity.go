package admind

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var errNoCompanyDirectory = errors.New("this host has no company directory configured")

func (service *Service) memberIDByEmail(ctx context.Context, email string) (string, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return "", fmt.Errorf("a person needs an email address")
	}
	client := service.centralPlane()
	if client == nil {
		return "", errNoCompanyDirectory
	}
	member, isKnown, errorValue := client.MemberByEmail(ctx, normalizedEmail)
	if errorValue != nil {
		return "", errorValue
	}
	if !isKnown {
		return "", fmt.Errorf("%s is not in the company directory", normalizedEmail)
	}
	memberID := strings.TrimSpace(member.MemberID)
	if memberID == "" {
		return "", fmt.Errorf("the company directory gave %s no member id", normalizedEmail)
	}
	return memberID, nil
}

func (service *Service) memberIDOrLocalPersonID(ctx context.Context, email string) (string, error) {
	memberID, errorValue := service.memberIDByEmail(ctx, email)
	if errorValue == nil {
		return memberID, nil
	}
	localPersonID, localError := service.localBlueclawPersonIDByEmail(ctx, email)
	if localError != nil || strings.TrimSpace(localPersonID) == "" {
		return "", errorValue
	}
	return strings.TrimSpace(localPersonID), nil
}

func (service *Service) personIDForMutation(ctx context.Context, email string, name string) (string, error) {
	localPersonID, errorValue := service.localBlueclawPersonIDByEmail(ctx, email)
	if errorValue == nil && strings.TrimSpace(localPersonID) != "" {
		return strings.TrimSpace(localPersonID), nil
	}
	return service.seatedMemberIDByEmail(ctx, email, name)
}

func (service *Service) seatedMemberIDByEmail(ctx context.Context, email string, name string) (string, error) {
	client := service.centralPlane()
	if client == nil {
		return "", errNoCompanyDirectory
	}
	member, errorValue := client.EnsureMember(ctx, email, name)
	if errorValue != nil {
		return "", errorValue
	}
	return strings.TrimSpace(member.MemberID), nil
}
