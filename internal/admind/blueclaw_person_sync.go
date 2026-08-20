package admind

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

func (service *Service) inviteBlueclawPerson(ctx context.Context, userID string, email string, name string) error {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return fmt.Errorf("userID required")
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return fmt.Errorf("email required")
	}
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return errorValue
	}
	people, _ := policyDocument["people"].([]any)
	for _, value := range people {
		if person, isPerson := value.(map[string]any); isPerson && blueclawPersonHasEmail(person, normalizedEmail) {
			return nil
		}
	}
	person := map[string]any{"personID": normalizedUserID, "emails": []string{normalizedEmail}}
	if strings.TrimSpace(name) != "" {
		person["displayName"] = strings.TrimSpace(name)
	}
	policyDocument["people"] = append(people, person)
	return service.deliverBlueclawPolicy(ctx, policyDocument)
}

func (service *Service) upsertBlueclawPerson(ctx context.Context, userID string, email string, name string, role string, circles []string, note *string) error {
	if errorValue := service.inviteBlueclawPerson(ctx, userID, email, name); errorValue != nil {
		return errorValue
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	return service.updateBlueclawPersonCircles(ctx, normalizedEmail, name, role, circles, note)
}

func (service *Service) updateBlueclawPersonCircles(ctx context.Context, email string, name string, role string, circles []string, note *string) error {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return errorValue
	}
	people, _ := policyDocument["people"].([]any)
	for _, value := range people {
		person, isPerson := value.(map[string]any)
		if !isPerson || !blueclawPersonHasEmail(person, email) {
			continue
		}
		person["circles"] = normalizeAdminUserCircles(circles, role)
		if strings.TrimSpace(name) != "" {
			person["displayName"] = strings.TrimSpace(name)
		}
		if note != nil {
			trimmedNote := strings.TrimSpace(*note)
			if trimmedNote == "" {
				delete(person, "note")
			} else {
				person["note"] = trimmedNote
			}
		}
		if normalizeAdminUserRole(role) == "admin" {
			person["isAdmin"] = true
			person["securityLevelName"] = "admin"
			person["securityLevelRank"] = 100
			person["grantedClasses"] = []string{"internal", "executive"}
		} else {
			person["isAdmin"] = false
			if strings.TrimSpace(mattermostPolicyString(person["securityLevelName"])) == "" || mattermostPolicyString(person["securityLevelName"]) == "admin" {
				person["securityLevelName"] = "member"
			}
			if rank, _ := person["securityLevelRank"].(float64); rank == 0 || rank == 100 {
				person["securityLevelRank"] = 10
			}
			person["grantedClasses"] = []string{"internal"}
		}
		break
	}
	return service.deliverBlueclawPolicy(ctx, policyDocument)
}
