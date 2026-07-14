package admind

import (
	"context"
	"strings"
)

func (service *Service) resolveLocalOrgchartMutationIdentity(
	ctx context.Context,
	payload adminUserMutation,
) (adminUserMutation, orgchartPersonIdentity, error) {
	userID := strings.TrimSpace(payload.UserID)
	if userID == "" {
		var errorValue error
		userID, errorValue = service.localBlueclawPersonIDByEmail(ctx, payload.Email)
		if errorValue != nil {
			return payload, orgchartPersonIdentity{}, errorValue
		}
		if userID == "" {
			userID = newInternKimUserID()
		}
	}
	payload.UserID = userID
	return payload, orgchartPersonIdentity{UserID: userID, Email: payload.Email}, nil
}

func (service *Service) resolveLocalOrgchartRemovalIdentity(ctx context.Context, email string) (orgchartPersonIdentity, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	userID, errorValue := service.localBlueclawPersonIDByEmail(ctx, normalizedEmail)
	if errorValue != nil {
		return orgchartPersonIdentity{}, errorValue
	}
	return orgchartPersonIdentity{UserID: userID, Email: normalizedEmail}, nil
}
