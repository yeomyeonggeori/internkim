package admind

import "testing"

func TestTheCircleEveryoneIsInAnswersToBothItsNames(t *testing.T) {
	for _, named := range []string{"member", "staff", "Staff", " member "} {
		if !isTheCircleEveryoneIsIn(named) {
			t.Errorf("%q is the circle everyone is in and was not recognised", named)
		}
	}
	for _, named := range []string{"", "admin", "hr", "c-level", "members"} {
		if isTheCircleEveryoneIsIn(named) {
			t.Errorf("%q is not the circle everyone is in", named)
		}
	}
}

// A person record written before the rename names the old circle. Normalising
// it must not leave them in two circles that mean the same people.
func TestAPersonIsNotLeftInTheCircleUnderItsOldName(t *testing.T) {
	normalized := normalizeAdminUserCircles([]string{"staff", "hr"}, "user")

	for _, circle := range normalized {
		if circle == formerMemberCircleName {
			t.Fatalf("the old name survived normalisation: %v", normalized)
		}
	}
	held := map[string]bool{}
	for _, circle := range normalized {
		if held[circle] {
			t.Fatalf("%s appears twice: %v", circle, normalized)
		}
		held[circle] = true
	}
	if !held["member"] || !held["hr"] {
		t.Fatalf("normalisation lost a circle: %v", normalized)
	}
}
