package personname

import (
	"strings"
	"unicode"
)

// A person's name is recorded one way and read another. The record holds it
// given name first, separated by spaces — 표본 김, John Michael Smith — because
// that is the one order every messenger, directory and mail header agrees on.
// Korean writes the family name first and joins it to the given name, so that
// is a rendering, not a second name to keep.
//
// Only a name written in Hangul is rejoined. "John Michael Smith" read by
// somebody with Korean selected is still John Michael Smith: the reader's
// language does not change how a Latin name is written.
//
// The web renders the same rule in TypeScript. person-name-cases.json is what
// both read, so neither can drift alone.
func Render(recorded string, language string) string {
	trimmed := strings.TrimSpace(recorded)
	parts := strings.Fields(trimmed)
	if !isKorean(language) || len(parts) < 2 || !isHangul(trimmed) {
		return trimmed
	}
	family := parts[len(parts)-1]
	return family + strings.Join(parts[:len(parts)-1], "")
}

func isKorean(language string) bool {
	return strings.EqualFold(strings.TrimSpace(language), "ko")
}

func isHangul(name string) bool {
	hasLetter := false
	for _, character := range name {
		if unicode.IsSpace(character) {
			continue
		}
		if !unicode.Is(unicode.Hangul, character) {
			return false
		}
		hasLetter = true
	}
	return hasLetter
}
