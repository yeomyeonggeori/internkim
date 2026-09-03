package admind

import "strings"

func normalizeCallingCode(callingCode string) string {
	return keepDigits(callingCode)
}

func keepDigits(value string) string {
	var digits strings.Builder
	for _, character := range value {
		if character >= '0' && character <= '9' {
			digits.WriteRune(character)
		}
	}
	return digits.String()
}
