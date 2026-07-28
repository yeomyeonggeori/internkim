package admind

import (
	"strings"
)

const (
	minimumInternationalPhoneNumberDigits = 8
	maximumInternationalPhoneNumberDigits = 15
)

func normalizeCallingCode(callingCode string) string {
	return keepDigits(callingCode)
}

func normalizeInternationalPhoneNumber(phoneNumber string, callingCode string) (string, error) {
	trimmedPhoneNumber := strings.TrimSpace(phoneNumber)
	if trimmedPhoneNumber == "" {
		return "", nil
	}
	digits := internationalPhoneNumberDigits(trimmedPhoneNumber, callingCode)
	if len(digits) < minimumInternationalPhoneNumberDigits || len(digits) > maximumInternationalPhoneNumberDigits {
		return "", organizationProfileInvalidRequestError("phone number must be a valid international number")
	}
	return "+" + digits, nil
}

func internationalPhoneNumberDigits(phoneNumber string, callingCode string) string {
	digits := keepDigits(phoneNumber)
	if digits == "" {
		return ""
	}
	if strings.HasPrefix(phoneNumber, "+") {
		return digits
	}
	if strings.HasPrefix(digits, "00") {
		return strings.TrimPrefix(digits, "00")
	}
	normalizedCallingCode := normalizeCallingCode(callingCode)
	if normalizedCallingCode == "" {
		normalizedCallingCode = workspaceDefaultCallingCode
	}
	if strings.HasPrefix(digits, normalizedCallingCode) && len(digits) > len(normalizedCallingCode) {
		return digits
	}
	return normalizedCallingCode + strings.TrimPrefix(digits, "0")
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
