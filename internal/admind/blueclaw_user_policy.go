package admind

import (
	"strings"
)

type adminCircleRecord struct {
	CircleID    string `json:"circleID"`
	DisplayName string `json:"displayName"`
}

func blueclawPersonHasEmail(person map[string]any, email string) bool {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	emailValues, _ := person["emails"].([]any)
	for _, value := range emailValues {
		candidate, isString := value.(string)
		if isString && strings.ToLower(strings.TrimSpace(candidate)) == normalizedEmail {
			return true
		}
	}
	return false
}

func blueclawAvailableCircles(policyDocument map[string]any) []adminCircleRecord {
	circleValues, _ := policyDocument["circles"].([]any)
	circles := []adminCircleRecord{}
	for _, value := range circleValues {
		circle, isCircle := value.(map[string]any)
		if !isCircle {
			continue
		}
		circleID := strings.ToLower(strings.TrimSpace(policyString(circle["circleID"])))
		if circleID == "" {
			continue
		}
		displayName := strings.TrimSpace(policyString(circle["displayName"]))
		if displayName == "" {
			displayName = circleID
		}
		circles = append(circles, adminCircleRecord{CircleID: circleID, DisplayName: displayName})
	}
	if len(circles) == 0 {
		return []adminCircleRecord{{CircleID: "member", DisplayName: "Member"}}
	}
	return circles
}

func blueclawCirclesByEmail(policyDocument map[string]any) map[string][]string {
	people, _ := policyDocument["people"].([]any)
	circlesByEmail := map[string][]string{}
	for _, value := range people {
		person, isPerson := value.(map[string]any)
		if !isPerson {
			continue
		}
		circles := policyStringList(person["circles"])
		emailValues, _ := person["emails"].([]any)
		for _, emailValue := range emailValues {
			email, isString := emailValue.(string)
			if isString {
				circlesByEmail[strings.ToLower(strings.TrimSpace(email))] = circles
			}
		}
	}
	return circlesByEmail
}
