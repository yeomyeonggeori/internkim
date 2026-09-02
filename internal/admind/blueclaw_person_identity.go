package admind

import (
	"context"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/buzzimport/mattermostadmin"
)

func (service *Service) localBlueclawPersonIDByEmail(ctx context.Context, email string) (string, error) {
	personIDs, errorValue := service.localBlueclawPersonIDsByEmail(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	return personIDs[strings.ToLower(strings.TrimSpace(email))], nil
}

func (service *Service) localBlueclawPersonIDsByEmail(ctx context.Context) (map[string]string, error) {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return nil, errorValue
	}
	people, _ := policyDocument["people"].([]any)
	personIDs := map[string]string{}
	for _, value := range people {
		person, isPerson := value.(map[string]any)
		if !isPerson {
			continue
		}
		personID := strings.TrimSpace(policyString(person["personID"]))
		if personID == "" {
			continue
		}
		emailValues, _ := person["emails"].([]any)
		for _, emailValue := range emailValues {
			email, isString := emailValue.(string)
			if !isString {
				continue
			}
			if normalized := strings.ToLower(strings.TrimSpace(email)); normalized != "" {
				personIDs[normalized] = personID
			}
		}
	}
	return personIDs, nil
}

func (service *Service) demoteLocalBlueclawPersonBeforeRemoval(ctx context.Context, userRecord mattermostadmin.UserRecord) error {
	personID, errorValue := service.localBlueclawPersonIDByEmail(ctx, userRecord.Email)
	if errorValue != nil {
		return errorValue
	}
	if personID == "" {
		return nil
	}
	name := firstNonEmpty(userRecord.Nickname, userRecord.DisplayName, userRecord.Username)
	return service.upsertBlueclawPerson(ctx, personID, userRecord.Email, name, "member", []string{"member"}, nil)
}

func (service *Service) localSaveBlueclawPerson(ctx context.Context, payload adminUserMutation, hasExplicitCircleMutation bool) error {
	if hasExplicitCircleMutation {
		return service.upsertBlueclawPerson(ctx, payload.MemberID, payload.Email, payload.Name, payload.Role, payload.Circles, &payload.Note)
	}
	return service.inviteBlueclawPerson(ctx, payload.MemberID, payload.Email, payload.Name)
}
