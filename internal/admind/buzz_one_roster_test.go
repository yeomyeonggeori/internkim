package admind

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestEveryKeyARoomIsFilledWithIsOneTheSweepKeeps(t *testing.T) {
	service := serviceWhoseDirectoryAndPolicyHold(t,
		`{"members":[{"memberID":"member-1","email":"seated@example.com","status":"active"}]}`,
		`{"people":[{"personID":"p-1","emails":["seated@example.com","second-address@example.com"]}]}`)

	members, errorValue := service.memberBuzzMembers(context.Background())
	if errorValue != nil {
		t.Fatalf("read who the company holds: %v", errorValue)
	}
	accounted, errorValue := service.accountedBuzzPubkeys(context.Background(), identityTestSeed)
	if errorValue != nil {
		t.Fatalf("read which keys the sweep keeps: %v", errorValue)
	}

	if len(members) == 0 {
		t.Fatal("the member the directory names must be seated")
	}
	for _, member := range members {
		if !accounted[member.Pubkey] {
			t.Fatalf("%s is seated as %s, a key the sweep takes back, so every pass adds it again and announces it", member.Email, member.Pubkey)
		}
	}
	secondAddressKey := derivedKey(t, versionedSubject("second-address@example.com", 1))
	for _, member := range members {
		if member.Pubkey == secondAddressKey {
			t.Fatalf("an address only the policy lists was seated: %s", member.Email)
		}
	}
}

func TestNobodyIsSeatedWhileTheDirectoryDoesNotAnswer(t *testing.T) {
	service := serviceHoldingTheSeed(t)
	pointAtACompanyDirectory(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, `{"people":[{"personID":"p-1","emails":["cached@example.com"]}]}`, nil), nil
		}
		return nil, errors.New("the directory is down")
	})}

	members, errorValue := service.memberBuzzMembers(context.Background())
	if errorValue == nil {
		t.Fatalf("a roster read while the directory is down must say so, not seat from a cache: %v", members)
	}
}
