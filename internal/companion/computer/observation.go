package computer

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Page struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type Reference struct {
	Ref        string   `json:"ref"`
	Role       string   `json:"role"`
	Name       string   `json:"name"`
	Value      string   `json:"value"`
	States     []string `json:"states"`
	Actions    []string `json:"actions"`
	Visibility string   `json:"visibility"`
}

type Observation struct {
	Page    Page        `json:"page"`
	Outline string      `json:"outline"`
	Refs    []Reference `json:"refs"`
}

const largestOutlineBytes = 12 * 1024

func parseObservation(document json.RawMessage) (Observation, error) {
	var observation Observation
	if errorValue := json.Unmarshal(document, &observation); errorValue != nil {
		return Observation{}, fmt.Errorf("browser state is not a semantic snapshot: %w", errorValue)
	}
	observation.Outline = truncateOutline(observation.Outline, largestOutlineBytes)
	return observation, nil
}

func truncateOutline(outline string, limit int) string {
	if len(outline) <= limit {
		return outline
	}
	cut := strings.LastIndexByte(outline[:limit], '\n')
	if cut <= 0 {
		cut = limit
	}
	return outline[:cut] + "\n… (outline truncated)"
}

func (reference Reference) can(action string) bool {
	for _, offered := range reference.Actions {
		if offered == action {
			return true
		}
	}
	return false
}

func (reference Reference) isVisible() bool {
	return reference.Visibility == "in_viewport" || reference.Visibility == "near_viewport"
}

func (reference Reference) label() string {
	name := strings.TrimSpace(reference.Name)
	if name == "" {
		name = strings.TrimSpace(reference.Value)
	}
	if name == "" {
		return reference.Role
	}
	return fmt.Sprintf("%s %q", reference.Role, shorten(name, 60))
}

func shorten(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}
