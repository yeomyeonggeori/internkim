package admind

import "testing"

func TestFleetAccountUpsertPayloadCarriesNoOrganizationFields(t *testing.T) {
	payload := fleetAccountUpsertPayload(adminUserMutation{
		UserID:      "user-member",
		Email:       "member@example.com",
		HireDate:    "2026-01-02",
		JobTitle:    "Product Manager",
		GroupID:     "product",
		PhoneNumber: "+821012345678",
	}, "fleet-1")

	for _, organizationField := range []string{"hireDate", "jobTitle", "groupID", "phoneNumber", "supervisorID", "positionLevel", "teamRole", "employmentStatus"} {
		if _, found := payload[organizationField]; found {
			t.Fatalf("account payload carries organization field %q; organization_profiles owns it", organizationField)
		}
	}
	if payload["email"] != "member@example.com" {
		t.Fatalf("account payload = %#v; want the account identity fields", payload)
	}
}
