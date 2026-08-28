package admind

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

func stableTaskID(value string) string {
	digest := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(digest[:])[:12]
}

func emailDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" || seen[trimmedValue] {
			continue
		}
		seen[trimmedValue] = true
		result = append(result, trimmedValue)
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	filteredValues := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			filteredValues = append(filteredValues, value)
		}
	}
	return filteredValues
}

func firstNonEmptySlice(values ...[]string) []string {
	for _, value := range values {
		cleanedValue := uniqueNonEmpty(value)
		if len(cleanedValue) > 0 {
			return cleanedValue
		}
	}
	return []string{}
}
