package capabilityd

import "strings"

const hintCandidateLimit = 20

type hintMatchable interface {
	hintID() string
	hintTitle() string
}

type hintResolution[Item hintMatchable] struct {
	Match       Item
	IsResolved  bool
	Candidates  []Item
	IsAmbiguous bool
}

func resolveHint[Item hintMatchable](hint string, items []Item, isPreferred func(Item) bool) hintResolution[Item] {
	trimmedHint := strings.TrimSpace(hint)
	if item, isFound := itemWithHintID(trimmedHint, items); isFound {
		return hintResolution[Item]{Match: item, IsResolved: true}
	}
	exactMatches := itemsWithTitleEqualTo(trimmedHint, items)
	if item, isFound := onlyOrPreferredItem(exactMatches, isPreferred); isFound {
		return hintResolution[Item]{Match: item, IsResolved: true}
	}
	if len(exactMatches) > 0 {
		return hintResolution[Item]{Candidates: limitedItems(exactMatches), IsAmbiguous: true}
	}
	containingMatches := itemsWithTitleContaining(trimmedHint, items)
	if item, isFound := onlyOrPreferredItem(containingMatches, isPreferred); isFound {
		return hintResolution[Item]{Match: item, IsResolved: true}
	}
	if len(containingMatches) > 0 {
		return hintResolution[Item]{Candidates: limitedItems(containingMatches), IsAmbiguous: true}
	}
	return hintResolution[Item]{Candidates: limitedItems(items)}
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

func itemWithHintID[Item hintMatchable](hintID string, items []Item) (Item, bool) {
	var noItem Item
	if hintID == "" {
		return noItem, false
	}
	for _, item := range items {
		if item.hintID() == hintID {
			return item, true
		}
	}
	return noItem, false
}

func itemsWithTitleEqualTo[Item hintMatchable](title string, items []Item) []Item {
	matches := make([]Item, 0, 1)
	for _, item := range items {
		if item.hintTitle() == title {
			matches = append(matches, item)
		}
	}
	return matches
}

func itemsWithTitleContaining[Item hintMatchable](title string, items []Item) []Item {
	collapsedTitle := collapseWhitespace(title)
	matches := make([]Item, 0, 1)
	if collapsedTitle == "" {
		return matches
	}
	for _, item := range items {
		if strings.Contains(collapseWhitespace(item.hintTitle()), collapsedTitle) {
			matches = append(matches, item)
		}
	}
	return matches
}

func collapseWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func limitedItems[Item hintMatchable](items []Item) []Item {
	if len(items) <= hintCandidateLimit {
		return items
	}
	return items[:hintCandidateLimit]
}

func unresolvedHintMessage(subject string, hintField string, identityField string, isAmbiguous bool) string {
	if isAmbiguous {
		return hintField + " matched more than one " + subject + "; retry with the exact " + identityField + " or exact title from one of the candidates"
	}
	return "no " + subject + " matched " + hintField + "; the candidates list what exists, so use one of them or take another route"
}
