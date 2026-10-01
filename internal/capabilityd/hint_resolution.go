package capabilityd

import (
	"sort"
	"strings"

	"gitlab.com/eastriver/internkim/internal/personname"
)

const approximateHintCandidateLimit = 8

type hintOutcome string

const (
	hintResolved    hintOutcome = "resolved"
	hintAmbiguous   hintOutcome = "ambiguous"
	hintApproximate hintOutcome = "approximate"
	hintNotFound    hintOutcome = "not_found"
)

// hintNearness answers how close a hint is to this item once nothing has
// matched it outright, on the scale that this kind of value actually fails on.
type hintMatchable interface {
	hintIdentifiers() []string
	hintTitle() string
	hintNearness(hint string) float64
}

type hintResolution[Item hintMatchable] struct {
	Outcome    hintOutcome
	Match      Item
	Candidates []Item
}

// An exact identifier is taken as given and a title only one item answers to
// is taken as meant; what a title answers to is personname's to say. Below
// that nothing is decided here: several matches or a near miss become choices
// for the person who asked, and an identifier is never approximated, because
// an identifier that is one character off was invented rather than mistyped.
func resolveHint[Item hintMatchable](hint string, items []Item, isPreferred func(Item) bool) hintResolution[Item] {
	trimmedHint := strings.TrimSpace(hint)
	if trimmedHint == "" {
		return hintResolution[Item]{Outcome: hintNotFound}
	}
	if item, isFound := itemWithHintIdentifier(trimmedHint, items); isFound {
		return hintResolution[Item]{Outcome: hintResolved, Match: item}
	}
	titleMatches := itemsWithTitleMatching(trimmedHint, items)
	if item, isFound := onlyOrPreferredItem(titleMatches, isPreferred); isFound {
		return hintResolution[Item]{Outcome: hintResolved, Match: item}
	}
	if len(titleMatches) > 0 {
		return hintResolution[Item]{Outcome: hintAmbiguous, Candidates: titleMatches}
	}
	if nearMatches := nearestItems(trimmedHint, items); len(nearMatches) > 0 {
		return hintResolution[Item]{Outcome: hintApproximate, Candidates: nearMatches}
	}
	return hintResolution[Item]{Outcome: hintNotFound}
}

func nearestItems[Item hintMatchable](hint string, items []Item) []Item {
	type scoredItem struct {
		item     Item
		nearness float64
		position int
	}
	scored := make([]scoredItem, 0, len(items))
	for position, item := range items {
		nearness := item.hintNearness(hint)
		if nearness <= 0 {
			continue
		}
		scored = append(scored, scoredItem{item: item, nearness: nearness, position: position})
	}
	sort.SliceStable(scored, func(first int, second int) bool {
		if scored[first].nearness != scored[second].nearness {
			return scored[first].nearness > scored[second].nearness
		}
		return scored[first].position < scored[second].position
	})
	if len(scored) > approximateHintCandidateLimit {
		scored = scored[:approximateHintCandidateLimit]
	}
	nearest := make([]Item, 0, len(scored))
	for _, entry := range scored {
		nearest = append(nearest, entry.item)
	}
	return nearest
}

func onlyOrPreferredItem[Item hintMatchable](items []Item, isPreferred func(Item) bool) (Item, bool) {
	var noItem Item
	if len(items) == 1 {
		return items[0], true
	}
	if len(items) == 0 || isPreferred == nil {
		return noItem, false
	}
	preferredItems := make([]Item, 0, 1)
	for _, item := range items {
		if isPreferred(item) {
			preferredItems = append(preferredItems, item)
		}
	}
	if len(preferredItems) != 1 {
		return noItem, false
	}
	return preferredItems[0], true
}

func itemWithHintIdentifier[Item hintMatchable](identifier string, items []Item) (Item, bool) {
	var noItem Item
	normalizedIdentifier := normalizedHintValue(identifier)
	if normalizedIdentifier == "" {
		return noItem, false
	}
	for _, item := range items {
		for _, candidateIdentifier := range item.hintIdentifiers() {
			if candidateIdentifier != "" && normalizedHintValue(candidateIdentifier) == normalizedIdentifier {
				return item, true
			}
		}
	}
	return noItem, false
}

func normalizedHintValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func itemsWithTitleMatching[Item hintMatchable](title string, items []Item) []Item {
	found := make([]Item, 0, 1)
	for _, item := range items {
		if personname.Matches(title, item.hintTitle()) {
			found = append(found, item)
		}
	}
	return found
}
