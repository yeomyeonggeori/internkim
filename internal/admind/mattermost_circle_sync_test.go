package admind

import (
	"testing"
)

// A circle channel that is not there says nothing about who belongs to the
// circle. Creating it to answer the question makes the answer "nobody", and the
// sync then takes the circle away from every person who had it.

func TestAPersonKeepsACircleWhoseChannelIsGone(t *testing.T) {
	person := map[string]any{
		"emails":  []any{"member@example.com"},
		"circles": []any{"member", "representative", "c-level"},
	}
	circleEmailsByID := map[string]map[string]bool{"c-level": {"member@example.com": true}}

	circles := mattermostSyncedPersonCircles(person, circleEmailsByID)

	if !containsCircle(circles, "representative") {
		t.Errorf("circles = %v, representative was dropped because its channel was not read", circles)
	}
	if !containsCircle(circles, "c-level") {
		t.Errorf("circles = %v", circles)
	}
}

func containsCircle(circles []string, circleID string) bool {
	for _, circle := range circles {
		if circle == circleID {
			return true
		}
	}
	return false
}
