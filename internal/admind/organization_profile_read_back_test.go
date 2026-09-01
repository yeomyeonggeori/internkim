package admind

import (
	"testing"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func TestOrganizationReadBackHoldsOffUntilTheDirectoryIsSeeded(t *testing.T) {
	described := []organizationProfile{{MemberID: "member-one", Email: "member@example.com", JobTitle: "Designer"}}
	blank := []centralplane.Member{{MemberID: "member-one", Email: "member@example.com"}}

	if !isDirectorySeedNeeded(blank, described) {
		t.Fatal("a directory that describes nobody must not be taken as an instruction to empty the device")
	}
	if isDirectorySeedNeeded(blank, nil) {
		t.Fatal("a device that describes nobody either has nothing to lose")
	}
	filled := []centralplane.Member{{MemberID: "member-one", Email: "member@example.com", JobTitle: "Designer"}}
	if isDirectorySeedNeeded(filled, described) {
		t.Fatal("a directory that describes somebody is the source and needs no seed")
	}
}

func TestOrganizationReadBackLeavesWhatTheDirectoryDoesNotOwn(t *testing.T) {
	held := organizationProfile{
		MemberID: "member-one", Email: "member@example.com",
		JobTitle: "Designer"}
	member := centralplane.Member{MemberID: "member-one", Email: "member@example.com", JobTitle: "Product Manager"}

	taken := organizationProfileTakenFrom(held, member, map[string]string{}, map[string]string{})

	if taken.JobTitle != "Product Manager" {
		t.Fatalf("job title = %q; the directory owns it", taken.JobTitle)
	}
}

func containsGroupNamed(groups []orgGroupRecord, name string) bool {
	for _, group := range groups {
		if group.Name == name {
			return true
		}
	}
	return false
}
