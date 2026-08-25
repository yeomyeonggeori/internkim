package capabilityd

import "strings"

const (
	typoNearnessFloor  = 0.5
	titleNearnessFloor = 0.34
)

// A short closed-vocabulary value fails by being mistyped, so nearness is how
// few edits separate it from what was asked for.
func typoNearness(hint string, value string) float64 {
	hintRunes := []rune(hint)
	valueRunes := []rune(value)
	if len(hintRunes) == 0 || len(valueRunes) == 0 {
		return 0
	}
	longest := len(hintRunes)
	if len(valueRunes) > longest {
		longest = len(valueRunes)
	}
	distance := runeEditDistance(hintRunes, valueRunes)
	if distance > allowedTypoDistance(longest) {
		return 0
	}
	return 1 - float64(distance)/float64(longest)
}

func allowedTypoDistance(length int) int {
	if length <= 4 {
		return 1
	}
	return 2
}

// A free-form title is remembered in pieces or paraphrased rather than
// mistyped, so nearness is how much of it the two strings share.
func titleNearness(hint string, value string) float64 {
	hintBigrams := characterBigrams(hint)
	valueBigrams := characterBigrams(value)
	if len(hintBigrams) == 0 || len(valueBigrams) == 0 {
		return 0
	}
	shared := 0
	remaining := map[string]int{}
	for _, bigram := range valueBigrams {
		remaining[bigram]++
	}
	for _, bigram := range hintBigrams {
		if remaining[bigram] > 0 {
			remaining[bigram]--
			shared++
		}
	}
	return 2 * float64(shared) / float64(len(hintBigrams)+len(valueBigrams))
}

// The local part is who the address belongs to and the domain is where everyone
// in one company already is, so a domain that differs on its own is a slip
// rather than a different person. Both parts differing is neither.
func emailNearness(hint string, email string) float64 {
	hintLocal, hintDomain, hintIsAddress := splitEmailAddress(hint)
	local, domain, isAddress := splitEmailAddress(email)
	if !hintIsAddress || !isAddress {
		return 0
	}
	if hintLocal == local {
		if hintDomain == domain {
			return 1
		}
		return typoNearness(hintDomain, domain)
	}
	if hintDomain != domain {
		return 0
	}
	return typoNearness(hintLocal, local)
}

func splitEmailAddress(value string) (string, string, bool) {
	local, domain, found := strings.Cut(value, "@")
	if !found || local == "" || domain == "" {
		return "", "", false
	}
	return local, domain, true
}

func runeEditDistance(first []rune, second []rune) int {
	previousRow := make([]int, len(second)+1)
	currentRow := make([]int, len(second)+1)
	for column := range previousRow {
		previousRow[column] = column
	}
	for row := 1; row <= len(first); row++ {
		currentRow[0] = row
		for column := 1; column <= len(second); column++ {
			substitutionCost := 1
			if first[row-1] == second[column-1] {
				substitutionCost = 0
			}
			currentRow[column] = minimumOf(
				previousRow[column]+1,
				currentRow[column-1]+1,
				previousRow[column-1]+substitutionCost,
			)
		}
		previousRow, currentRow = currentRow, previousRow
	}
	return previousRow[len(second)]
}

func minimumOf(values ...int) int {
	smallest := values[0]
	for _, value := range values[1:] {
		if value < smallest {
			smallest = value
		}
	}
	return smallest
}

func characterBigrams(value string) []string {
	runes := []rune(strings.Join(strings.Fields(value), ""))
	if len(runes) < 2 {
		return nil
	}
	bigrams := make([]string, 0, len(runes)-1)
	for index := 0; index+1 < len(runes); index++ {
		bigrams = append(bigrams, string(runes[index:index+2]))
	}
	return bigrams
}
