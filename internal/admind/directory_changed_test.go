package admind

import (
	"net/http"
	"testing"
)

func TestAProfileGoesWhenTheDirectoryNoLongerHoldsThePerson(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" {
			return jsonResponse(http.StatusOK, `{"records":[{"memberID":"member-1","email":"stays@example.com","name":"Stays","role":"member"}]}`, nil), nil
		}
		return jsonResponse(http.StatusNotFound, `{}`, nil), nil
	})}
	if errorValue := service.writeOrganizationProfiles(t.Context(), []organizationProfile{
		{MemberID: "member-1", Email: "stays@example.com", JobTitle: "연구원"},
		{MemberID: "member-2", Email: "left@example.com", JobTitle: "인턴"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	service.forgetProfilesOfWhoeverLeft(t.Context())

	profiles, errorValue := service.readOrganizationProfiles(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if describesEmail(profiles, "left@example.com") {
		t.Fatalf("the profile of somebody the directory dropped stayed: %+v", profiles)
	}
	if !describesEmail(profiles, "stays@example.com") {
		t.Fatalf("a colleague's profile was forgotten: %+v", profiles)
	}
}

// A directory that cannot be read is not a directory that holds nobody.
func TestNoProfileIsForgottenWhenTheDirectoryDoesNotAnswer(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusBadGateway, `{}`, nil), nil
	})}
	if errorValue := service.writeOrganizationProfiles(t.Context(), []organizationProfile{
		{MemberID: "member-1", Email: "stays@example.com", JobTitle: "연구원"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	service.forgetProfilesOfWhoeverLeft(t.Context())

	profiles, errorValue := service.readOrganizationProfiles(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !describesEmail(profiles, "stays@example.com") {
		t.Fatalf("a profile was forgotten on an unreadable directory: %+v", profiles)
	}
}
