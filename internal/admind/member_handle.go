package admind

import (
	"strings"
	"unicode"
)

const shortestMemberHandle = 3
const longestMemberHandle = 22

func memberHandleBase(email string) string {
	localPart := strings.Split(email, "@")[0]
	var builder strings.Builder
	for _, character := range strings.ToLower(localPart) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '.' {
			builder.WriteRune(character)
			continue
		}
		builder.WriteByte('-')
	}
	handle := strings.Trim(builder.String(), "-_.")
	if len(handle) < shortestMemberHandle || !isLowercaseASCIIAlpha(rune(handle[0])) {
		handle = "user-" + randomHex(3)
	}
	if len(handle) > longestMemberHandle {
		handle = handle[:longestMemberHandle]
	}
	return handle
}

func normalizeMemberHandle(handle string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(strings.TrimSpace(handle)) {
		if isLowercaseASCIIAlpha(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '.' {
			builder.WriteRune(character)
		}
	}
	return strings.Trim(builder.String(), "-_.")
}

func isValidMemberHandle(handle string) bool {
	if len(handle) < shortestMemberHandle || len(handle) > longestMemberHandle {
		return false
	}
	if !isLowercaseASCIIAlpha(rune(handle[0])) {
		return false
	}
	for _, character := range handle {
		if isLowercaseASCIIAlpha(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func isLowercaseASCIIAlpha(character rune) bool {
	return character >= 'a' && character <= 'z'
}
