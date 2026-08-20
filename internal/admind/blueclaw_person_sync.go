package admind

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

func (service *Service) inviteBlueclawPerson(ctx context.Context, userID string, email string, name string) error {
	return service.upsertBlueclawPerson(ctx, userID, email, name, "", nil, nil)
}

// upsertBlueclawPerson reads the roster once, applies everything about the person, and
// writes once. Two read-modify-write passes were a race with our own first write: the
// second read has to see it, and on a device the agent reads the file across a share, so
// a stale read silently dropped the person the first pass had just added.
func (service *Service) upsertBlueclawPerson(ctx context.Context, userID string, email string, name string, role string, circles []string, note *string) error {
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
	person := blueclawPersonWithEmail(people, normalizedEmail)
	if person == nil {
		person = map[string]any{"personID": normalizedUserID, "emails": []string{normalizedEmail}}
		people = append(people, person)
		policyDocument["people"] = people
	}
	applyBlueclawPersonAttributes(person, name, role, circles, note)
	return service.deliverBlueclawPolicy(ctx, policyDocument)
}

func blueclawPersonWithEmail(people []any, email string) map[string]any {
	for _, value := range people {
		if person, isPerson := value.(map[string]any); isPerson && blueclawPersonHasEmail(person, email) {
			return person
		}
	}
	return nil
}

func applyBlueclawPersonAttributes(person map[string]any, name string, role string, circles []string, note *string) {
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
		return
	}
	person["isAdmin"] = false
	if strings.TrimSpace(mattermostPolicyString(person["securityLevelName"])) == "" || mattermostPolicyString(person["securityLevelName"]) == "admin" {
		person["securityLevelName"] = "member"
	}
	if rank, _ := person["securityLevelRank"].(float64); rank == 0 || rank == 100 {
		person["securityLevelRank"] = 10
	}
	person["grantedClasses"] = []string{"internal"}
}
