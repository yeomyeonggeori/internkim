package admind

import "testing"

func TestTheFormerCircleNameLeavesAndTheRestStay(t *testing.T) {
	kept := circlesWithoutTheFormerName([]string{"staff", "member", "admin", "hr"})

	if len(kept) != 3 {
		t.Fatalf("expected the other three to stay: %v", kept)
	}
	for _, circle := range kept {
		if circle == formerMemberCircleName {
			t.Fatalf("the old name survived: %v", kept)
		}
	}
	if kept[0] != "member" || kept[1] != "admin" || kept[2] != "hr" {
		t.Fatalf("the order of the circles that stay changed: %v", kept)
	}
}

func TestAPersonWithoutTheFormerCircleIsLeftAlone(t *testing.T) {
	circles := []string{"member", "c-level"}

	kept := circlesWithoutTheFormerName(circles)

	if len(kept) != len(circles) {
		t.Fatalf("a record that needed nothing was changed: %v", kept)
	}
}
