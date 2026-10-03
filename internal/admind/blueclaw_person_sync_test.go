package admind

import "testing"

func TestAPersonWrittenToThePolicyDropsTheRetiredSecurityLevel(t *testing.T) {
	person := map[string]any{
		"securityLevelName": "admin",
		"securityLevelRank": float64(100),
		"grantedClasses":    []any{"internal", "executive"},
		"isAdmin":           true,
	}

	applyBlueclawPersonAttributes(person, "이샘플", "member", nil, nil, "ko")

	for _, retired := range []string{"securityLevelName", "securityLevelRank", "grantedClasses"} {
		if _, isHeld := person[retired]; isHeld {
			t.Fatalf("%s is still written; blueclaw stopped reading it, so it only leaves a number nobody checks", retired)
		}
	}
	if person["isAdmin"] != false {
		t.Fatalf("isAdmin = %v for a member", person["isAdmin"])
	}
}
