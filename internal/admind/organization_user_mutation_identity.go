package admind

import (
	"context"
	"strings"
)

func (service *Service) resolveLocalOrganizationMutationIdentity(
	ctx context.Context,
	payload adminUserMutation,
) (adminUserMutation, organizationPersonIdentity, error) {
	memberID, errorValue := service.personIDForMutation(ctx, payload.Email, payload.Name)
	if errorValue != nil {
		return payload, organizationPersonIdentity{}, errorValue
	}
	payload.MemberID = memberID
	return payload, organizationPersonIdentity{MemberID: memberID, Email: payload.Email}, nil
}

func (service *Service) resolveLocalOrganizationRemovalIdentity(ctx context.Context, email string, fallbackMemberID string) (organizationPersonIdentity, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	memberID, errorValue := service.memberIDOrLocalPersonID(ctx, normalizedEmail)
	if errorValue != nil {
		memberID = strings.TrimSpace(fallbackMemberID)
	}
	return organizationPersonIdentity{MemberID: memberID, Email: normalizedEmail}, nil
}

func organizationProxyMutationIdentities(sourceMemberID string, canonicalIdentity organizationPersonIdentity) []organizationPersonIdentity {
	identities := []organizationPersonIdentity{canonicalIdentity}
	normalizedSourceMemberID := strings.TrimSpace(sourceMemberID)
	if normalizedSourceMemberID == "" || normalizedSourceMemberID == strings.TrimSpace(canonicalIdentity.MemberID) {
		return identities
	}
	return append(identities, organizationPersonIdentity{MemberID: normalizedSourceMemberID, Email: canonicalIdentity.Email})
}
