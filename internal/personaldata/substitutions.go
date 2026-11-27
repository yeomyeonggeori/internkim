package personaldata

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	shortestTokenDigest  = 8
	fewestDigitsInAValue = 9
)

var tokenPattern = regexp.MustCompile(`\{\{[A-Za-z]+:[0-9a-f]{8,64}\}\}`)

type Masker struct {
	key []byte
}

func NewMasker() Masker {
	key := make([]byte, 32)
	rand.Read(key)
	return Masker{key: key}
}

type Substitutions struct {
	valueByToken      map[string]string
	digitsByToken     map[string]string
	maskedCountByKind map[string]int
	restoredCount     int
	inventedCount     int
}

func NewSubstitutions() *Substitutions {
	return &Substitutions{valueByToken: map[string]string{}, digitsByToken: map[string]string{}, maskedCountByKind: map[string]int{}}
}

func (masker Masker) MaskText(text string, substitutions *Substitutions) string {
	var masked []byte
	copiedUntil := 0
	for start := 0; start < len(text); {
		if !isNumberCharacter(text[start]) {
			start++
			continue
		}
		end := endOfNumberRun(text, start)
		if digitCount(text[start:end]) >= fewestDigitsInAValue {
			from, to := widenedByOneRune(text, max(start, copiedUntil), end)
			from = max(from, copiedUntil)
			if segment := masker.maskSegment(text[from:to], substitutions); segment != text[from:to] {
				masked = append(append(masked, text[copiedUntil:from]...), segment...)
				copiedUntil = to
			}
		}
		start = end
	}
	if masked == nil {
		return text
	}
	return string(append(masked, text[copiedUntil:]...))
}

func (masker Masker) maskSegment(text string, substitutions *Substitutions) string {
	for _, recognized := range kinds {
		text = recognized.pattern.ReplaceAllStringFunc(text, func(match string) string {
			if !recognized.isValid(match) {
				return match
			}
			substitutions.maskedCountByKind[recognized.name]++
			return masker.tokenFor(recognized.name, match, substitutions)
		})
	}
	return text
}

func (masker Masker) tokenFor(kindName string, value string, substitutions *Substitutions) string {
	digits := onlyDigits(value)
	digest := masker.digestOf(kindName + "\x00" + digits)
	for length := shortestTokenDigest; ; length += 2 {
		token := "{{" + kindName + ":" + digest[:length] + "}}"
		recordedDigits, isTaken := substitutions.digitsByToken[token]
		if !isTaken {
			substitutions.digitsByToken[token] = digits
			substitutions.valueByToken[token] = value
			return token
		}
		if recordedDigits == digits {
			return token
		}
	}
}

func (masker Masker) digestOf(text string) string {
	mac := hmac.New(sha256.New, masker.key)
	mac.Write([]byte(text))
	return hex.EncodeToString(mac.Sum(nil))
}

func (substitutions *Substitutions) IsEmpty() bool {
	return len(substitutions.valueByToken) == 0
}

func (substitutions *Substitutions) UnmaskText(text string) string {
	if !strings.Contains(text, "{{") {
		return text
	}
	return tokenPattern.ReplaceAllStringFunc(text, func(token string) string {
		value, isKnown := substitutions.valueByToken[token]
		if !isKnown {
			substitutions.inventedCount++
			return token
		}
		substitutions.restoredCount++
		return value
	})
}

func (substitutions *Substitutions) MaskedSummary() string {
	names := make([]string, 0, len(substitutions.maskedCountByKind))
	for name := range substitutions.maskedCountByKind {
		names = append(names, name)
	}
	slices.Sort(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s=%d", name, substitutions.maskedCountByKind[name]))
	}
	return strings.Join(parts, " ")
}

func (substitutions *Substitutions) RestoredSummary() string {
	return fmt.Sprintf("restored=%d invented=%d", substitutions.restoredCount, substitutions.inventedCount)
}
