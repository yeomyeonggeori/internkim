// Package personaldata keeps personal values out of what leaves this computer
// for a model provider. A value a pattern recognizes is sent as a token, and
// each token the model writes back is restored before anyone here reads it, so
// tools, the ledger and the messenger see the value while the model never does.
package personaldata

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

type kind struct {
	name     string
	pattern  *regexp.Regexp
	isValid  func(match string) bool
	identity func(match string) string
}

var keyKinds = []kind{
	{
		name: "apiKey",
		pattern: regexp.MustCompile(`\b(?:` + strings.Join([]string{
			`sk-(?:proj-|ant-(?:api|admin)\d{2}-|or-v1-)?[A-Za-z0-9_-]{20,}`,
			`gh[pousr]_[A-Za-z0-9]{36,}`,
			`github_pat_[A-Za-z0-9_]{60,}`,
			`AKIA[0-9A-Z]{16}`,
			`AIza[0-9A-Za-z_-]{35}`,
			`xox[abposr]-[A-Za-z0-9-]{10,}`,
			`[sr]k_live_[A-Za-z0-9]{20,}`,
			`hf_[A-Za-z0-9]{30,}`,
			`ik_[0-9a-f]{64}`,
			`eyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{16,}`,
		}, "|") + `)`),
		isValid:  isAnything,
		identity: itself,
	},
}

var keyMarkers = []string{"sk-", "ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_", "AKIA", "AIza", "xox", "k_live_", "hf_", "ik_", "eyJ"}

var numberKinds = []kind{
	{
		name:     "residentRegistrationNumber",
		pattern:  regexp.MustCompile(`\b\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])-?[1-8]\d{6}\b`),
		isValid:  isResidentRegistrationNumber,
		identity: onlyDigits,
	},
	{
		name:     "cardNumber",
		pattern:  regexp.MustCompile(`\b[3-6]\d{3}(?:[- ]?\d{4}){3}\b`),
		isValid:  passesLuhnCheck,
		identity: onlyDigits,
	},
	{
		name:     "phoneNumber",
		pattern:  regexp.MustCompile(`(?:\+82[- ]?|\b0)1[016789][- .]?\d{3,4}[- .]?\d{4}\b|\b0(?:2|[3-6][1-5]|70)[- .]\d{3,4}[- .]\d{4}\b`),
		isValid:  isAnything,
		identity: onlyDigits,
	},
}

func isAnything(string) bool { return true }

func itself(match string) string { return match }

var keyMarkersByFirstByte = func() [256][]string {
	var table [256][]string
	for _, marker := range keyMarkers {
		table[marker[0]] = append(table[marker[0]], marker)
	}
	return table
}()

func containsKeyMarker[Text string | []byte](text Text) bool {
	for position := range len(text) {
		for _, marker := range keyMarkersByFirstByte[text[position]] {
			if hasMarkerAt(text, position, marker) {
				return true
			}
		}
	}
	return false
}

func hasMarkerAt[Text string | []byte](text Text, position int, marker string) bool {
	if len(text)-position < len(marker) {
		return false
	}
	for offset := range len(marker) {
		if text[position+offset] != marker[offset] {
			return false
		}
	}
	return true
}

// Numbers issued since October 2020 carry no check digit (행정안전부 주민등록번호
// 부여체계 개편), so a written separator alone is enough to call one a number.
func isResidentRegistrationNumber(match string) bool {
	if strings.Contains(match, "-") {
		return true
	}
	return hasResidentRegistrationCheckDigit(digitsOf(match))
}

func hasResidentRegistrationCheckDigit(digits []int) bool {
	weights := []int{2, 3, 4, 5, 6, 7, 8, 9, 2, 3, 4, 5}
	sum := 0
	for position, weight := range weights {
		sum += digits[position] * weight
	}
	return (11-sum%11)%10 == digits[12]
}

func passesLuhnCheck(match string) bool {
	digits := digitsOf(match)
	sum := 0
	for position := range digits {
		digit := digits[len(digits)-1-position]
		if position%2 == 1 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	return sum%10 == 0
}

func isNumberCharacter(character byte) bool {
	return character >= '0' && character <= '9' || character == '-' || character == ' ' || character == '.' || character == '+'
}

func endOfNumberRun[Text string | []byte](text Text, start int) int {
	end := start
	for end < len(text) && isNumberCharacter(text[end]) {
		end++
	}
	return end
}

func digitCount[Text string | []byte](text Text) int {
	count := 0
	for position := range len(text) {
		if text[position] >= '0' && text[position] <= '9' {
			count++
		}
	}
	return count
}

func widenedByOneRune(text string, start int, end int) (int, int) {
	if start > 0 {
		_, size := utf8.DecodeLastRuneInString(text[:start])
		start -= size
	}
	if end < len(text) {
		_, size := utf8.DecodeRuneInString(text[end:])
		end += size
	}
	return start, end
}

func onlyDigits(text string) string {
	return strings.Map(func(character rune) rune {
		if character >= '0' && character <= '9' {
			return character
		}
		return -1
	}, text)
}

func digitsOf(text string) []int {
	digits := make([]int, 0, len(text))
	for _, character := range text {
		if character >= '0' && character <= '9' {
			digits = append(digits, int(character-'0'))
		}
	}
	return digits
}
