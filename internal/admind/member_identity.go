package admind

import (
	"context"
	"errors"
	"strings"
)

var errNoCompanyDirectory = errors.New("this host has no company directory configured")

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
