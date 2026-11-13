package personname

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A person's name is recorded one way and read another. The record holds it
// given name first, separated by spaces — 예시 김, John Michael Smith — because
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
	return strings.Join(familyFirst(parts), "")
}

type Tier int

const (
	Unreached Tier = iota
	ExactName
	PartOfName
	FamilyNameAndJobTitle
	JobTitle
)

type Person struct {
	Name     string
	JobTitle string
}

func ReachOf(hint string, person Person) Tier {
	hintWords := wordsOf(strings.TrimPrefix(strings.TrimSpace(hint), "@"))
	nameWords := wordsOf(person.Name)
	if len(hintWords) == 0 {
		return Unreached
	}
	if tier := nameTier(hintWords, nameWords); tier != Unreached {
		return tier
	}
	return jobTitleTier(strings.Join(hintWords, ""), nameWords, compactWords(person.JobTitle))
}

func ReferenceNearness(hint string, person Person) float64 {
	compactHint := compactWords(strings.TrimPrefix(strings.TrimSpace(hint), "@"))
	if compactHint == "" {
		return 0
	}
	nameWords := wordsOf(person.Name)
	covered := max(leadingNameFormLength(compactHint, nameWords), containedJobTitleLength(compactHint, nameWords, compactWords(person.JobTitle)))
	return float64(covered) / float64(utf8.RuneCountInString(compactHint))
}

func nameTier(hintWords []string, nameWords []string) Tier {
	if len(nameWords) == 0 {
		return Unreached
	}
	if isExactName(hintWords, nameWords) {
		return ExactName
	}
	if isPartOfName(hintWords, nameWords) {
		return PartOfName
	}
	return Unreached
}

func isExactName(hintWords []string, nameWords []string) bool {
	return slices.Equal(sortedWords(hintWords), sortedWords(nameWords)) ||
		strings.Join(hintWords, "") == joinedName(nameWords)
}

func isPartOfName(hintWords []string, nameWords []string) bool {
	if everyWordAmong(hintWords, nameWords) {
		return true
	}
	joined := joinedName(nameWords)
	return strings.Contains(joined, strings.Join(hintWords, "")) || strings.Contains(joined, strings.Join(familyFirst(hintWords), ""))
}

func jobTitleTier(compactHint string, nameWords []string, compactTitle string) Tier {
	if compactTitle == "" {
		return Unreached
	}
	if compactHint == compactTitle {
		return JobTitle
	}
	familyName := familyNameOf(nameWords)
	if familyName != "" && (compactHint == familyName+compactTitle || compactHint == compactTitle+familyName) {
		return FamilyNameAndJobTitle
	}
	return Unreached
}

func leadingNameFormLength(compactHint string, nameWords []string) int {
	longest := 0
	for _, form := range nameForms(nameWords) {
		if strings.HasPrefix(compactHint, form) {
			longest = max(longest, utf8.RuneCountInString(form))
		}
	}
	return longest
}

func containedJobTitleLength(compactHint string, nameWords []string, compactTitle string) int {
	if compactTitle == "" || !strings.Contains(compactHint, compactTitle) {
		return 0
	}
	remainder := strings.Replace(compactHint, compactTitle, "", 1)
	if familyName := familyNameOf(nameWords); familyName != "" && strings.Contains(remainder, familyName) {
		return utf8.RuneCountInString(compactTitle + familyName)
	}
	return utf8.RuneCountInString(compactTitle)
}

func nameForms(nameWords []string) []string {
	if len(nameWords) < 2 {
		return []string{joinedName(nameWords)}
	}
	return []string{joinedName(nameWords), strings.Join(nameWords[:len(nameWords)-1], "")}
}

func joinedName(nameWords []string) string {
	if len(nameWords) < 2 {
		return strings.Join(nameWords, "")
	}
	return strings.Join(familyFirst(nameWords), "")
}

func familyNameOf(nameWords []string) string {
	if len(nameWords) < 2 {
		return ""
	}
	return nameWords[len(nameWords)-1]
}

func wordsOf(text string) []string {
	return strings.Fields(strings.ToLower(text))
}

func compactWords(text string) string {
	return strings.Join(wordsOf(text), "")
}

func sortedWords(words []string) []string {
	sorted := slices.Clone(words)
	slices.Sort(sorted)
	return sorted
}

func everyWordAmong(words []string, among []string) bool {
	for _, word := range words {
		if !slices.Contains(among, word) {
			return false
		}
	}
	return true
}

func familyFirst(parts []string) []string {
	family := parts[len(parts)-1]
	return append([]string{family}, parts[:len(parts)-1]...)
}

func FirstName(recorded string) string {
	parts := strings.Fields(recorded)
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], " ")
}

func DefaultCallMe(recorded string, language string) string {
	firstName := FirstName(recorded)
	if firstName == "" {
		return ""
	}
	if isKorean(language) {
		return firstName + " 님"
	}
	return firstName
}

func isKorean(language string) bool {
	baseLanguage := strings.SplitN(strings.ToLower(strings.TrimSpace(language)), "-", 2)[0]
	return baseLanguage == "ko"
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
