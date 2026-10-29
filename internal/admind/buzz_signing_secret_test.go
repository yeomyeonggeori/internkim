package admind

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

func TestASigningSecretReadsTheDirectoryOnce(t *testing.T) {
	service := serviceHoldingTheSeed(t)
	pointAtACompanyDirectory(t, service)
	var directoryReads atomic.Int32
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isTheDirectoryAsked(request) {
			directoryReads.Add(1)
			return jsonResponse(http.StatusOK, `{"members":[
				{"memberID":"member-active","email":"active@example.com","status":"active"},
				{"memberID":"member-departed","email":"departed@example.com","status":"departed"}
			]}`, nil), nil
		}
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		}
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	})}

	for _, email := range []string{"active@example.com", "departed@example.com"} {
		directoryReads.Store(0)
		secretHex, errorValue := service.buzzSigningSecretForPubkey(context.Background(), derivedKey(t, versionedSubject(email, 1)))
		if errorValue != nil || secretHex != buzzidentity.Secret(identityTestSeed, versionedSubject(email, 1)) {
			t.Fatalf("expected the signing secret of %s, got %q %v", email, secretHex, errorValue)
		}
		if reads := directoryReads.Load(); reads != 1 {
			t.Fatalf("expected one read of the company directory for %s's key, got %d", email, reads)
		}
	}
}
