package admind

import (
	"encoding/json"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

var organizationFieldsForTest = []string{"hireDate", "jobTitle", "groupID", "teamID", "phoneNumber", "supervisorID", "supervisorEmail", "positionLevel", "teamRole", "employmentStatus"}

func TestAccountWriteCarriesNoOrganizationFields(t *testing.T) {
	document, errorValue := json.Marshal(centralplane.MemberWrite{
		Email:     "member@example.com",
		Name:      "이샘플",
		Role:      "member",
		Note:      "HR compensation follow-up",
		Messenger: map[string]string{"buzz": "a-buzz-key"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var written map[string]any
	if errorValue := json.Unmarshal(document, &written); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, organizationField := range organizationFieldsForTest {
		if _, found := written[organizationField]; found {
			t.Fatalf("the account write carries organization field %q; organization_profiles owns it", organizationField)
		}
	}
	if written["email"] != "member@example.com" || written["role"] != "member" {
		t.Fatalf("account write = %#v; want the account identity fields", written)
	}
}

func TestAccountRecordOfCompanyMemberKeepsOrganizationFieldsOut(t *testing.T) {
	record := accountRecordOfCompanyMember(centralplane.Member{
		MemberID:        "user-member",
		Email:           "Member@Example.com",
		Name:            "이샘플",
		Note:            "HR compensation follow-up",
		Role:            "admin",
		Circles:         []string{"admin"},
		Status:          "active",
		JobTitle:        "Product Manager",
		PhoneNumber:     "+821012345678",
		HireDate:        "2026-01-02",
		TeamID:          "product",
		TeamName:        "Product",
		SupervisorEmail: "lead@example.com",
	})

	document, errorValue := json.Marshal(record)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var written map[string]any
	if errorValue := json.Unmarshal(document, &written); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, organizationField := range organizationFieldsForTest {
		if _, found := written[organizationField]; found {
			t.Fatalf("the account record carries organization field %q; the organization reader merges it in", organizationField)
		}
	}
	if record.MemberID != "user-member" || record.Email != "member@example.com" || record.Handle != "member" ||
		record.Name != "이샘플" || record.Note != "HR compensation follow-up" || record.Role != "admin" ||
		record.Status != "active" || len(record.Circles) != 1 {
		t.Fatalf("account record = %+v; want the account identity fields", record)
	}
}
