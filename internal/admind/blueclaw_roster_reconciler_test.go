package admind

import (
	"testing"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func directoryOf(entries ...centralplane.Member) []centralplane.Member { return entries }

func TestOnlyTheBootstrapAccountMeansTheProjectionWasLost(t *testing.T) {
	if !holdsOnlyBootstrapAccount(map[string]bool{bootstrapPersonEmail: true}) {
		t.Fatal("a host serving a real company holds people beside the account it was born with, so holding it alone is the lost-projection state")
	}
	if holdsOnlyBootstrapAccount(map[string]bool{bootstrapPersonEmail: true, "이샘플@example.com": true}) {
		t.Fatal("a roster with a real person in it has not been lost")
	}
	if holdsOnlyBootstrapAccount(map[string]bool{}) {
		t.Fatal("a host that never built a roster is a fresh device, which is a different state from one that lost it")
	}
}

func TestAPersonTheDirectoryHasAndTheAgentDoesNotIsProjected(t *testing.T) {
	directory := directoryOf(
		centralplane.Member{MemberID: "member-1", Email: "이샘플@example.com", Status: "active"},
		centralplane.Member{MemberID: "member-2", Email: "박예시@example.com", Status: "active"},
	)

	missing := membersMissingFrom(directory, map[string]bool{"이샘플@example.com": true})

	if len(missing) != 1 || missing[0].Email != "박예시@example.com" {
		t.Fatalf("the point of deriving is that whoever the directory added reaches the agent, got %+v", missing)
	}
}

func TestAPersonAlreadyOnTheAgentIsNotProjectedAgain(t *testing.T) {
	directory := directoryOf(centralplane.Member{MemberID: "member-1", Email: "이샘플@example.com", Status: "active"})

	if missing := membersMissingFrom(directory, map[string]bool{"이샘플@example.com": true}); len(missing) != 0 {
		t.Fatalf("re-sending everyone every ten minutes writes to the agent for no reason, got %+v", missing)
	}
}

func TestACaseDifferentAddressIsTheSamePerson(t *testing.T) {
	directory := directoryOf(centralplane.Member{MemberID: "member-1", Email: "이샘플@example.com", Status: "active"})

	if missing := membersMissingFrom(directory, map[string]bool{"이샘플@example.com": true}); len(missing) != 0 {
		t.Fatal("addresses are matched case-insensitively, so a case difference must not create a second projection of one person")
	}
}

func TestAMemberWhoIsNoLongerActiveIsNotProjected(t *testing.T) {
	directory := directoryOf(
		centralplane.Member{MemberID: "member-1", Email: "이샘플@example.com", Status: "active"},
		centralplane.Member{MemberID: "member-2", Email: "떠난사람@example.com", Status: "removed"},
	)

	active := activeMembersOf(directory)

	if len(active) != 1 || active[0].Email != "이샘플@example.com" {
		t.Fatalf("someone the directory stopped calling active must not keep reaching the agent, got %+v", active)
	}
}
