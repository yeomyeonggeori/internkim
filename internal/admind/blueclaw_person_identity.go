package admind

import (
	"context"
	"net/http"
	"strings"
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
