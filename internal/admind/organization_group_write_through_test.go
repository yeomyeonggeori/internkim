package admind

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func TestRenamingATeamInTheDirectoryDoesNotMakeASecondGroup(t *testing.T) {
	held := []orgGroupRecord{{ID: "team-product", Name: "제품"}}
	members := []centralplane.Member{{Email: "one@example.com", TeamID: "team-product", TeamName: "프로덕트"}}

	renamed := groupsRenamedByTheDirectory(held, members)
	if len(renamed) != 1 || renamed[0].ID != "team-product" || renamed[0].Name != "프로덕트" {
		t.Fatalf("groups = %#v; a team keeps its id when it is renamed", renamed)
	}

	groupIDByName := map[string]string{strings.ToLower(renamed[0].Name): renamed[0].ID}
	heldIDs := map[string]bool{"team-product": true}
	missing := groupsTheDirectoryNamesAndTheDeviceLacks(members, groupIDByName, heldIDs)
	if len(missing) != 0 {
		t.Fatalf("missing = %#v; the renamed team is not a team the device lacks", missing)
	}
}

func TestGroupsAdoptTheTeamIDsTheDirectoryIssues(t *testing.T) {
	groups := []orgGroupRecord{{ID: "product", Name: "제품"}, {ID: "design", Name: "디자인", ParentID: "product"}}
	settled := []centralplane.Team{{TeamID: "team-product", Name: "제품"}, {TeamID: "team-design", Name: "디자인"}}

	adopted, renamedIDs := groupsAdoptingTeamIDs(groups, settled)

	if len(adopted) != 2 || adopted[0].ID != "team-product" || adopted[1].ID != "team-design" {
		t.Fatalf("adopted = %#v", adopted)
	}
	if adopted[1].ParentID != "team-product" {
		t.Fatalf("parent = %q; the hierarchy has to move with the ids", adopted[1].ParentID)
	}
	if renamedIDs["product"] != "team-product" || renamedIDs["design"] != "team-design" {
		t.Fatalf("renamed = %#v; people pointing at the old ids need to be moved", renamedIDs)
	}
}
