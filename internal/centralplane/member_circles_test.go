package centralplane

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestAMemberReadsTheCirclesTheCompanySends(t *testing.T) {
	var member Member
	document := `{"memberID":"m1","email":"lee@example.test","role":"admin","circles":["staff","c-level"]}`
	if errorValue := json.Unmarshal([]byte(document), &member); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Equal(member.Circles, []string{"staff", "c-level"}) {
		t.Fatalf("the field name the company sends must be the one this reads, got %v", member.Circles)
	}
}

func TestAMemberTheCompanyPutsInNoCircleCarriesNone(t *testing.T) {
	var member Member
	if errorValue := json.Unmarshal([]byte(`{"memberID":"m1","email":"lee@example.test"}`), &member); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(member.Circles) != 0 {
		t.Fatalf("an absent field must read as no circles, got %v", member.Circles)
	}
}
