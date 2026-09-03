package admind

import "testing"

func newOrganizationProfileTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(Configuration{StateDirectory: t.TempDir()})
}

func TestForgettingAProfileLeavesEveryoneElseDescribed(t *testing.T) {
	service := newOrganizationProfileTestService(t)
	if errorValue := service.writeOrganizationProfiles(t.Context(), []organizationProfile{
		{MemberID: "member-1", Email: "stays@example.com", JobTitle: "연구원"},
		{MemberID: "member-2", Email: "leaves@example.com", JobTitle: "인턴"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.forgetOrganizationProfile(t.Context(), "leaves@example.com", "member-2"); errorValue != nil {
		t.Fatal(errorValue)
	}

	profiles, errorValue := service.readOrganizationProfiles(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if describesEmail(profiles, "leaves@example.com") || !describesEmail(profiles, "stays@example.com") {
		t.Fatalf("profiles = %+v", profiles)
	}
}

func TestForgettingAProfileNamingNobodyChangesNothing(t *testing.T) {
	service := newOrganizationProfileTestService(t)
	if errorValue := service.writeOrganizationProfiles(t.Context(), []organizationProfile{
		{MemberID: "member-1", Email: "stays@example.com", JobTitle: "연구원"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.forgetOrganizationProfile(t.Context(), "", ""); errorValue != nil {
		t.Fatal(errorValue)
	}

	profiles, errorValue := service.readOrganizationProfiles(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !describesEmail(profiles, "stays@example.com") {
		t.Fatalf("profiles = %+v", profiles)
	}
}

func describesEmail(profiles []organizationProfile, email string) bool {
	for _, profile := range profiles {
		if profile.Email == email {
			return true
		}
	}
	return false
}
