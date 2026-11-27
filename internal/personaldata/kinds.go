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
	name    string
	pattern *regexp.Regexp
	isValid func(match string) bool
}

var kinds = []kind{
	{
		name:    "residentRegistrationNumber",
		pattern: regexp.MustCompile(`\b\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])-?[1-8]\d{6}\b`),
		isValid: isResidentRegistrationNumber,
	},
	{
		name:    "cardNumber",
		pattern: regexp.MustCompile(`\b[3-6]\d{3}(?:[- ]?\d{4}){3}\b`),
		isValid: passesLuhnCheck,
	},
	{
		name:    "phoneNumber",
		pattern: regexp.MustCompile(`(?:\+82[- ]?|\b0)1[016789][- .]?\d{3,4}[- .]?\d{4}\b|\b0(?:2|[3-6][1-5]|70)[- .]\d{3,4}[- .]\d{4}\b`),
		isValid: func(string) bool { return true },
	},
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
