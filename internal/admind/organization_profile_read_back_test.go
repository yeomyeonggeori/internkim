package admind

import (
	"context"
	"net/http"
	"testing"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func TestOrganizationReadBackTakesTheDirectorysProfile(t *testing.T) {
	service := newLocalUsersTestService(t)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if response, isHandled := localOrganizationDirectoryResponse(t, request); isHandled {
			return response, nil
		}
		if isCompanyDirectoryRequest(request) {
			return jsonResponse(http.StatusOK, `{"members":[
				{"memberID":"member-lead","email":"lead@example.com","role":"admin","status":"active","teamID":"team-product","teamName":"Product"},
				{"memberID":"member-one","email":"member@example.com","role":"member","status":"active","jobTitle":"Designer","phoneNumber":"+821012345678","hireDate":"2026-01-02","teamID":"team-product","teamName":"Product","supervisorEmail":"lead@example.com"}
			]}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	service.readOrganizationProfilesBackFromTheDirectory(context.Background())

	profilesByEmail, errorValue := service.readOrganizationProfilesByEmail(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	profile := profilesByEmail["member@example.com"]
	if profile.JobTitle != "Designer" {
		t.Fatalf("job title = %q; the directory's profile never reached the device", profile.JobTitle)
	}
	if profile.HireDate != "2026-01-02" || profile.PhoneNumber != "+821012345678" {
		t.Fatalf("profile = %#v", profile)
	}
	if profile.SupervisorID != "member-lead" {
		t.Fatalf("supervisor = %q; want the member the directory named by email", profile.SupervisorID)
	}
	if profile.GroupID != "team-product" {
		t.Fatalf("group = %q; want the team the directory named", profile.GroupID)
	}

	groups, errorValue := service.readOrganizationGroups(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !containsGroupNamed(groups, "Product") {
		t.Fatalf("groups = %#v; a team the directory names has to exist here to be pointed at", groups)
	}
}

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
