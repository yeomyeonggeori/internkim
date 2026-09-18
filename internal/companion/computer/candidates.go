package computer

import (
	"fmt"
	"strings"
)

// Jev's Choice question accepts at most 32 criteria; two are always reserved.
const largestCandidateTable = 32

const (
	ReobserveCandidateID = "reobserve"
	AbstainCandidateID   = "abstain"
)

type Input struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

type Candidate struct {
	ID          string
	Description string
	Tool        string
	Arguments   map[string]any
}

func (candidate Candidate) isReserved() bool {
	return candidate.Tool == ""
}

type candidateTable struct {
	candidates []Candidate
	byID       map[string]Candidate
}

func (table candidateTable) criteria() map[string]string {
	criteria := make(map[string]string, len(table.candidates))
	for _, candidate := range table.candidates {
		criteria[candidate.ID] = candidate.Description
	}
	return criteria
}

func (table candidateTable) lookup(id string) (Candidate, bool) {
	candidate, isKnown := table.byID[id]
	return candidate, isKnown
}

func buildCandidates(observation Observation, inputs []Input) candidateTable {
	actions := typingCandidates(observation.Refs, inputs)
	actions = append(actions, clickCandidates(observation.Refs)...)
	actions = append(actions, scrollCandidates(observation.Refs)...)
	actions = capCandidates(actions, largestCandidateTable-2)
	candidates := append(actions, reservedCandidates()...)
	byID := make(map[string]Candidate, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	return candidateTable{candidates: candidates, byID: byID}
}

func capCandidates(candidates []Candidate, limit int) []Candidate {
	if len(candidates) <= limit {
		return candidates
	}
	return candidates[:limit]
}

func reservedCandidates() []Candidate {
	return []Candidate{
		{ID: ReobserveCandidateID, Description: "Take a fresh look at the page without acting; choose this when the page may still be changing or the listed actions do not match what the goal needs yet."},
		{ID: AbstainCandidateID, Description: "Stop without acting because none of the listed actions is safe or useful for reaching the goal from this page."},
	}
}

func typingCandidates(refs []Reference, inputs []Input) []Candidate {
	candidates := []Candidate{}
	for _, reference := range refs {
		if !reference.can("type") || !reference.isVisible() {
			continue
		}
		for index, input := range inputs {
			candidates = append(candidates, Candidate{
				ID:          typingCandidateID(index, input, reference),
				Description: fmt.Sprintf("Replace the contents of the %s with the %s text.", reference.label(), input.Name),
				Tool:        "browser_type",
				Arguments:   map[string]any{"ref": reference.Ref, "text": input.Text, "replace": true},
			})
		}
		candidates = append(candidates, Candidate{
			ID:          "enter-in-" + reference.Ref,
			Description: fmt.Sprintf("Press Enter in the %s to submit what it holds.", reference.label()),
			Tool:        "browser_type",
			Arguments:   map[string]any{"ref": reference.Ref, "text": "\n"},
		})
	}
	return candidates
}

func clickCandidates(refs []Reference) []Candidate {
	candidates := []Candidate{}
	for _, reference := range refs {
		if !reference.can("click") || !reference.isVisible() {
			continue
		}
		candidates = append(candidates, Candidate{
			ID:          "click-" + reference.Ref,
			Description: "Click the " + reference.label() + ".",
			Tool:        "browser_click",
			Arguments:   map[string]any{"ref": reference.Ref, "input_route": "dom_event"},
		})
	}
	return candidates
}

func scrollCandidates(refs []Reference) []Candidate {
	scrollable, isFound := firstScrollable(refs)
	if !isFound {
		return nil
	}
	return []Candidate{
		{
			ID:          "scroll-down",
			Description: "Scroll the page down to reveal what is below the visible part.",
			Tool:        "browser_pointer",
			Arguments:   map[string]any{"ref": scrollable.Ref, "action": "scroll", "input_route": "dom_event", "delta_y": 600},
		},
		{
			ID:          "scroll-up",
			Description: "Scroll the page up to reveal what is above the visible part.",
			Tool:        "browser_pointer",
			Arguments:   map[string]any{"ref": scrollable.Ref, "action": "scroll", "input_route": "dom_event", "delta_y": -600},
		},
	}
}

func firstScrollable(refs []Reference) (Reference, bool) {
	for _, reference := range refs {
		if reference.can("scroll") {
			return reference, true
		}
	}
	for _, reference := range refs {
		if reference.can("pointer") {
			return reference, true
		}
	}
	return Reference{}, false
}

func typingCandidateID(index int, input Input, reference Reference) string {
	return fmt.Sprintf("type-%d%s-into-%s", index+1, slug(input.Name), reference.Ref)
}

func slug(name string) string {
	letters := strings.Map(func(character rune) rune {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			return character
		}
		if character >= 'A' && character <= 'Z' {
			return character + ('a' - 'A')
		}
		return -1
	}, name)
	if letters == "" {
		return ""
	}
	return "-" + shorten(letters, 24)
}
