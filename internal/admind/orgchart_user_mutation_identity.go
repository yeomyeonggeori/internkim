package admind

import (
	"context"
	"strings"
)

func (service *Service) resolveLocalOrgchartMutationIdentity(
	ctx context.Context,
	payload adminUserMutation,
) (adminUserMutation, orgchartPersonIdentity, error) {
	userID, errorValue := service.localBlueclawPersonIDByEmail(ctx, payload.Email)
	if errorValue != nil {
		return payload, orgchartPersonIdentity{}, errorValue
	}
	if userID == "" {
		userID = strings.TrimSpace(payload.UserID)
	}
	if userID == "" {
		userID = newInternKimUserID()
	}
	payload.UserID = userID
	return payload, orgchartPersonIdentity{UserID: userID, Email: payload.Email}, nil
}

func (service *Service) resolveLocalOrgchartRemovalIdentity(ctx context.Context, email string, fallbackUserID string) (orgchartPersonIdentity, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	userID, errorValue := service.localBlueclawPersonIDByEmail(ctx, normalizedEmail)
	if errorValue != nil {
		return orgchartPersonIdentity{}, errorValue
	}
	if userID == "" {
		userID = strings.TrimSpace(fallbackUserID)
	}
	return orgchartPersonIdentity{UserID: userID, Email: normalizedEmail}, nil
}
