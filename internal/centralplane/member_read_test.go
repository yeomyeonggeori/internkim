package centralplane

import (
	"context"
	"strings"
	"testing"
)

func TestTheDirectoryIsReadWithBothAnswersForEveryMember(t *testing.T) {
	client, _ := memberDoorForTest(t, `{"members":[
		{"memberID":"m1","email":"one@example.com","name":"이샘플","status":"active","isWorking":true,"canReadTheCompany":true},
		{"memberID":"m2","email":"two@example.com","name":"박예시","status":"departed","isWorking":false,"canReadTheCompany":false}
	]}`)

	members, errorValue := client.Members(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(members) != 2 || !members[0].IsWorking || !members[0].CanReadTheCompany || members[0].Name != "이샘플" {
		t.Fatalf("members = %+v; somebody working arrives as working, with the rest of what the directory says", members)
	}
	if members[1].IsWorking || members[1].CanReadTheCompany {
		t.Fatalf("members[1] = %+v; a no the directory said is a no", members[1])
	}
}

func TestADirectoryThatDoesNotSayWhetherAMemberWorksHereIsRefused(t *testing.T) {
	unsaid := map[string]string{
		"neither answer":       `{"memberID":"m1","email":"one@example.com","status":"active"}`,
		"no isWorking":         `{"memberID":"m1","email":"one@example.com","status":"active","canReadTheCompany":true}`,
		"no canReadTheCompany": `{"memberID":"m1","email":"one@example.com","status":"active","isWorking":true}`,
	}
	for name, member := range unsaid {
		t.Run(name, func(t *testing.T) {
			everyone, _ := memberDoorForTest(t, `{"members":[`+member+`]}`)
			if members, errorValue := everyone.Members(context.Background()); errorValue == nil || !strings.Contains(errorValue.Error(), "m1") {
				t.Fatalf("members = %+v, error = %v; an answer nobody gave is not a no, and the refusal names the member", members, errorValue)
			}

			one, _ := memberDoorForTest(t, `{"member":`+member+`}`)
			if found, isKnown, errorValue := one.MemberByEmail(context.Background(), "one@example.com"); errorValue == nil {
				t.Fatalf("member = %+v, known = %v; one address is read by the same rule", found, isKnown)
			}
		})
	}
}

func TestAnAddressTheDirectoryDoesNotHoldIsStillNobody(t *testing.T) {
	client, _ := memberDoorForTest(t, `{"member":null}`)

	_, isKnown, errorValue := client.MemberByEmail(context.Background(), "nobody@example.com")
	if errorValue != nil || isKnown {
		t.Fatalf("known = %v, error = %v; nobody at an address is an answer, not a refusal", isKnown, errorValue)
	}
}
