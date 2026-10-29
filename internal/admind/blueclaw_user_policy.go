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
