package admind

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/personname"
)

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
	language := service.workspaceLanguage(ctx)
	applyBlueclawPersonAttributes(person, name, role, circles, note, language)
	if errorValue := service.deliverBlueclawPolicy(ctx, policyDocument); errorValue != nil {
		return errorValue
	}
	service.seedUserDocument(ctx, policyString(person["personID"]), personname.DefaultCallMe(name, language))
	return nil
}

func blueclawPersonWithEmail(people []any, email string) map[string]any {
	for _, value := range people {
		if person, isPerson := value.(map[string]any); isPerson && blueclawPersonHasEmail(person, email) {
			return person
		}
	}
	return nil
}

func applyBlueclawPersonAttributes(person map[string]any, name string, role string, circles []string, note *string, language string) {
	person["circles"] = normalizeAdminUserCircles(circles, role)
	if displayName := personname.Render(name, language); displayName != "" {
		person["displayName"] = displayName
	}
	if note != nil {
		trimmedNote := strings.TrimSpace(*note)
		if trimmedNote == "" {
			delete(person, "note")
		} else {
			person["note"] = trimmedNote
		}
	}
	person["isAdmin"] = normalizeAdminUserRole(role) == "admin"
	for _, retired := range retiredPersonFields {
		delete(person, retired)
	}
}

var retiredPersonFields = []string{"securityLevelName", "securityLevelRank", "grantedClasses"}
