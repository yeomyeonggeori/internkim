package admind

import "testing"

func TestFleetAccountUpsertPayloadCarriesNoOrganizationFields(t *testing.T) {
	payload := fleetAccountUpsertPayload(adminUserMutation{
		MemberID:    "user-member",
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

func TestFleetAccountUpsertPayloadCarriesOnlyTheAccountIdentity(t *testing.T) {
	payload := fleetAccountUpsertPayload(adminUserMutation{
		MemberID: "user-member",
		Handle:   "member",
		Name:     "이샘플",
		Email:    "member@example.com",
		Role:     "member",
		Status:   "active",
	}, "fleet-1")

	accountFields := map[string]bool{
		"handle":   true,
		"name":     true,
		"note":     true,
		"fleet_id": true,
		"email":    true,
		"role":     true,
		"status":   true,
	}
	for field := range payload {
		if !accountFields[field] {
			t.Fatalf("account payload carries %q; the account directory holds sign-in, and member.messenger holds who somebody is on a messenger", field)
		}
	}
	for field := range accountFields {
		if _, found := payload[field]; !found {
			t.Fatalf("account payload = %#v; want the account identity field %q", payload, field)
		}
	}
}
