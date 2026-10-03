package admind

import (
	"strings"
)

func normalizeAdminUserCircles(circles []string, role string) []string {
	normalizedCircles := []string{"member"}
	if normalizeAdminUserRole(role) == "admin" {
		normalizedCircles = append(normalizedCircles, "admin")
	}
	for _, circle := range circles {
		normalizedCircle := strings.ToLower(strings.TrimSpace(circle))
		if normalizedCircle == "" || isTheCircleEveryoneIsIn(normalizedCircle) {
			continue
		}
		normalizedCircles = append(normalizedCircles, normalizedCircle)
	}
	return uniqueAdminStrings(normalizedCircles)
}

func uniqueAdminStrings(values []string) []string {
	seenValue := map[string]bool{}
	result := []string{}
	for _, value := range values {
		normalizedValue := strings.ToLower(strings.TrimSpace(value))
		if normalizedValue == "" || seenValue[normalizedValue] {
			continue
		}
		seenValue[normalizedValue] = true
		result = append(result, normalizedValue)
	}
	return result
}

// declareTheCirclesPeopleHold makes the policy's circles exactly the ones
// somebody holds. Circles live on the central plane, so the
// people decide which circles exist here; admin is a role, not a place.
func declareTheCirclesPeopleHold(policyDocument map[string]any) {
	existing := map[string]map[string]any{}
	for _, value := range anySlice(policyDocument["circles"]) {
		if circle, isCircle := value.(map[string]any); isCircle {
			existing[strings.ToLower(strings.TrimSpace(policyString(circle["circleID"])))] = circle
		}
	}
	held := []string{memberCircleName}
	for _, value := range anySlice(policyDocument["people"]) {
		if person, isPerson := value.(map[string]any); isPerson {
			held = append(held, policyStringList(person["circles"])...)
		}
	}
	declared := []any{}
	for _, circleID := range uniqueAdminStrings(held) {
		if circleID == "admin" {
			continue
		}
		circle := existing[circleID]
		if circle == nil {
			circle = map[string]any{"circleID": circleID, "displayName": circleID}
		}
		circle["workspaceDirectoryPath"] = "/workspace/circles/" + circleID
		declared = append(declared, circle)
	}
	policyDocument["circles"] = declared
}

func anySlice(value any) []any {
	values, _ := value.([]any)
	return values
}
