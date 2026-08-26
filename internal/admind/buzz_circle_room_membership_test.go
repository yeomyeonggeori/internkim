package admind

import (
	"slices"
	"strings"
	"testing"
)

func TestACircleRoomIsFoundByTheNameThePolicyDeclares(t *testing.T) {
	circles := declaredCirclesOfPolicy(map[string]any{
		"circles": []any{
			map[string]any{"circleID": "c-level", "displayName": "C-level"},
			map[string]any{"circleID": "hr", "displayName": "HR"},
			map[string]any{"circleID": "", "displayName": "Nameless"},
			map[string]any{"circleID": "staff"},
		},
	})

	if len(circles) != 2 {
		t.Fatalf("a circle with no id and one with no display name name no room, got %+v", circles)
	}
	if circles[1].CircleID != "hr" || circles[1].DisplayName != "HR" {
		t.Fatalf("expected the hr circle to carry both, got %+v", circles[1])
	}
}

func TestOnlyThePeopleCarryingTheCircleBelong(t *testing.T) {
	circlesByEmail := map[string][]string{
		"lee@example.test":  {"staff", "admin", "c-level"},
		"rain@example.test": {"staff", "admin"},
		"kwak@example.test": {"staff"},
	}

	belong := emailsCarrying(circlesByEmail, "c-level")
	slices.Sort(belong)
	if !slices.Equal(belong, []string{"lee@example.test"}) {
		t.Fatalf("a room for the c-level circle holds the c-level circle, got %v", belong)
	}
}

func TestACircleNobodyCarriesHoldsNobody(t *testing.T) {
	belong := emailsCarrying(map[string][]string{"lee@example.test": {"staff"}}, "representative")

	if len(belong) != 0 {
		t.Fatalf("an empty circle names nobody rather than everybody, got %v", belong)
	}
}

func TestTheStaffSyncOnlyEverFillsRoomsTheWholeCompanyCanRead(t *testing.T) {
	query := staffRoomQuery

	for _, condition := range []string{"visibility = 'open'", "deleted_at IS NULL", "created_by = decode($1, 'hex')"} {
		if !strings.Contains(query, condition) {
			t.Fatalf("a room the whole company is added to must be %s: %s", condition, query)
		}
	}
}
