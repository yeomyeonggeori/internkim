package admind

import (
	"encoding/json"
	"testing"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

// The account directory holds who somebody is and how they sign in.
// organization_profiles holds what they do here. A write that carries both makes
// two owners for one field, and the one that wrote last wins by accident.
func TestAccountWriteCarriesNoOrganizationFields(t *testing.T) {
	document, errorValue := json.Marshal(centralplane.MemberWrite{
		Email:     "member@example.com",
		Name:      "이샘플",
		Role:      "member",
		Note:      "joined through the invite",
		Messenger: map[string]string{"mattermost": "mm-1"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var payload map[string]any
	if errorValue := json.Unmarshal(document, &payload); errorValue != nil {
		t.Fatal(errorValue)
	}

	for _, organizationField := range []string{"hireDate", "jobTitle", "groupID", "teamID", "phoneNumber", "supervisorID", "supervisorEmail", "positionLevel", "teamRole", "employmentStatus"} {
		if _, found := payload[organizationField]; found {
			t.Fatalf("account write carries organization field %q; organization_profiles owns it", organizationField)
		}
	}
	if payload["email"] != "member@example.com" {
		t.Fatalf("account write = %#v; want the account identity fields", payload)
	}
}

// The record a caller reads back is the same boundary from the other side.
func TestAccountRecordOfCompanyMemberKeepsOrganizationFieldsOut(t *testing.T) {
	record := accountRecordOfCompanyMember(centralplane.Member{
		MemberID:        "member-1",
		Email:           "Member@Example.com",
		Name:            "이샘플",
		Role:            "admin",
		Status:          "active",
		JobTitle:        "Product Manager",
		PhoneNumber:     "+821012345678",
		HireDate:        "2026-01-02",
		TeamID:          "team-1",
		SupervisorEmail: "lead@example.com",
	})

	if record.Email != "member@example.com" || record.MemberID != "member-1" {
		t.Fatalf("account record = %#v; want the account identity fields", record)
	}
	if record.JobTitle != "" || record.PhoneNumber != "" || record.HireDate != "" || record.GroupID != "" || record.SupervisorID != "" {
		t.Fatalf("account record carries organization fields: %#v", record)
	}
}
