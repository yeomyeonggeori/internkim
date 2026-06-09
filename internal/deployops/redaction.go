package deployops

import (
	"regexp"
	"strings"
)

var secretAssignmentPattern = regexp.MustCompile(`(?i)(secret|token|password|authorization)([=: ]+)([^[:space:]]+)`)
var longSecretPattern = regexp.MustCompile(`[A-Za-z0-9_~./+=-]{32,}`)

func Redact(value string) string {
	value = secretAssignmentPattern.ReplaceAllString(value, `$1$2[redacted]`)
	return longSecretPattern.ReplaceAllStringFunc(value, func(candidate string) string {
		if strings.HasPrefix(candidate, "http://") || strings.HasPrefix(candidate, "https://") {
			return candidate
		}
		if strings.Contains(candidate, "/") {
			return candidate
		}
		return "[redacted]"
	})
}
