package admind

import (
	"context"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"

	nostr "github.com/nbd-wtf/go-nostr"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

func TestEveryKeyHeldByCarriesEveryVersionAPersonHasHad(t *testing.T) {
	seed := "3c" + "00000000000000000000000000000000000000000000000000000000000000"
	email := "sample@example.com"
	service := &Service{}
	service.Configuration.StateDirectory = t.TempDir()
	service.Configuration.BlueclawBaseURL = "http://127.0.0.1:8080"
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	held, errorValue := service.everyKeyHeldBy(context.Background(), seed, email)
	if errorValue != nil {
		t.Fatalf("every key held by: %v", errorValue)
	}
	if len(held) != 1 {
		t.Fatalf("a person who was never rotated holds one key: %v", held)
	}

	if _, errorValue := service.bumpBuzzIdentityVersion(email); errorValue != nil {
		t.Fatalf("bump: %v", errorValue)
	}
	held, errorValue = service.everyKeyHeldBy(context.Background(), seed, email)
	if errorValue != nil {
		t.Fatalf("every key held by: %v", errorValue)
	}
	if len(held) != 2 {
		t.Fatalf("a rotated person holds both keys: %v", held)
	}

	current, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, versionedSubject(email, 2)))
	if errorValue != nil {
		t.Fatalf("derive: %v", errorValue)
	}
	if held[1] != current {
		t.Fatalf("the newest version is the current key: %v", held)
	}
	if _, errorValue := filepath.Abs(service.buzzIdentityVersionPath(email)); errorValue != nil {
		t.Fatalf("version path: %v", errorValue)
	}
}

// A key nobody derives is shown out of the community, and the one exception is
// a key that has said something: taking that one out would take its messages
// out of the conversations they belong to, which is a person's call.
func TestAKeyThatSaidSomethingIsLeftForAPersonToJudge(t *testing.T) {
	unaccounted := []string{"aa", "bb", "cc"}
	spoke := map[string]bool{"bb": true}

	safe := keysSafeToShowOut(unaccounted, spoke)

	if !reflect.DeepEqual(safe, []string{"aa", "cc"}) {
		t.Errorf("keysSafeToShowOut = %v, want [aa cc]", safe)
	}
}

func TestAKeyThatSaidNothingIsShownOut(t *testing.T) {
	safe := keysSafeToShowOut([]string{"aa"}, map[string]bool{})
	if !reflect.DeepEqual(safe, []string{"aa"}) {
		t.Errorf("keysSafeToShowOut = %v, want [aa]", safe)
	}
}

func TestEveryUnaccountedKeyHavingSpokenLeavesNothingToSweep(t *testing.T) {
	safe := keysSafeToShowOut([]string{"aa", "bb"}, map[string]bool{"aa": true, "bb": true})
	if len(safe) != 0 {
		t.Errorf("keysSafeToShowOut = %v, want none", safe)
	}
}
