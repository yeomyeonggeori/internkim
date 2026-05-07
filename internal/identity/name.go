package identity

import "strings"

func SplitNameForMattermost(name string) (firstName string, lastName string) {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) >= 2 && IsHangulSyllable(runes[0]) {
		remaining := strings.Join(strings.Fields(string(runes[1:])), "")
		if remaining != "" {
			return remaining, string(runes[:1])
		}
		return string(runes[:1]), ""
	}
	parts := strings.Fields(name)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], parts[len(parts)-1]
	}
}

func CallingName(name string) string {
	firstName, _ := SplitNameForMattermost(name)
	return firstName
}

func NicknameForMattermost(name string) string {
	canonicalName := strings.TrimSpace(name)
	if canonicalName == "" {
		return ""
	}
	runes := []rune(canonicalName)
	if len(runes) > 0 && IsHangulSyllable(runes[0]) {
		return strings.Join(strings.Fields(canonicalName), "")
	}
	firstName, _ := SplitNameForMattermost(canonicalName)
	return firstName
}

func IsHangulSyllable(value rune) bool {
	return value >= 0xAC00 && value <= 0xD7A3
}
