package admind

import (
	"context"
	"strings"
)

func (service *Service) resolveLocalOrganizationMutationIdentity(
	ctx context.Context,
	payload adminUserMutation,
) (adminUserMutation, organizationPersonIdentity, error) {
	userID, errorValue := service.localBlueclawPersonIDByEmail(ctx, payload.Email)
	if errorValue != nil {
		return payload, organizationPersonIdentity{}, errorValue
	}
	if userID == "" {
		userID = strings.TrimSpace(payload.UserID)
	}
	if userID == "" {
		userID = newInternKimUserID()
	}
	payload.UserID = userID
	return payload, organizationPersonIdentity{UserID: userID, Email: payload.Email}, nil
}

func (service *Service) resolveLocalOrganizationRemovalIdentity(ctx context.Context, email string, fallbackUserID string) (organizationPersonIdentity, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	userID, errorValue := service.localBlueclawPersonIDByEmail(ctx, normalizedEmail)
	if errorValue != nil {
		return organizationPersonIdentity{}, errorValue
	}
	if userID == "" {
		userID = strings.TrimSpace(fallbackUserID)
	}
	return organizationPersonIdentity{UserID: userID, Email: normalizedEmail}, nil
}

func organizationProxyMutationIdentities(sourceUserID string, canonicalIdentity organizationPersonIdentity) []organizationPersonIdentity {
	identities := []organizationPersonIdentity{canonicalIdentity}
	normalizedSourceUserID := strings.TrimSpace(sourceUserID)
	if normalizedSourceUserID == "" || normalizedSourceUserID == strings.TrimSpace(canonicalIdentity.UserID) {
		return identities
	}
	return append(identities, organizationPersonIdentity{UserID: normalizedSourceUserID, Email: canonicalIdentity.Email})
}
