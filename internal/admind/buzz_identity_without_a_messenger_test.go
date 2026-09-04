package admind

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

const identityTestSeed = "3c" + "00000000000000000000000000000000000000000000000000000000000000"

func serviceHoldingTheSeed(t *testing.T) *Service {
	t.Helper()
	service := &Service{}
	service.Configuration.StateDirectory = t.TempDir()
	service.Configuration.BlueclawBaseURL = "http://127.0.0.1:8080"
	seedPath := filepath.Join(service.Configuration.StateDirectory, "buzz-key-seed")
	if errorValue := writeFileAtomically(seedPath, []byte(identityTestSeed), 0o600); errorValue != nil {
		t.Fatalf("write seed: %v", errorValue)
	}
	service.Configuration.BuzzKeySeedPath = seedPath
	return service
}

func serviceWhoseRecordAnswers(t *testing.T, policyDocument string) *Service {
	t.Helper()
	service := serviceHoldingTheSeed(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, policyDocument, nil), nil
		}
		if strings.Contains(request.URL.String(), ":8065") {
			t.Fatalf("the identity path asked a messenger: %s", request.URL.String())
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
	return service
}

func serviceWhoseRecordIsDown(t *testing.T) *Service {
	t.Helper()
	service := serviceHoldingTheSeed(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("blueclaw is down")
	})}
	return service
}

func TestPersonBuzzSecretResolvesWithNoMessengerAnywhere(t *testing.T) {
	service := serviceWhoseRecordAnswers(t, `{"people":[{"personID":"p-1","emails":["sample@example.com"]}]}`)

	secretHex, errorValue := service.personBuzzSecret(context.Background(), "sample@example.com")
	if errorValue != nil {
		t.Fatalf("person buzz secret: %v", errorValue)
	}
	if secretHex != buzzidentity.Secret(identityTestSeed, versionedSubject("sample@example.com", 1)) {
		t.Fatalf("the key is not the one the record says: %s", secretHex)
	}
}

func TestPersonBuzzSecretRefusesWhenTheRecordIsUnreachable(t *testing.T) {
	service := serviceWhoseRecordIsDown(t)

	secretHex, errorValue := service.personBuzzSecret(context.Background(), "sample@example.com")
	if errorValue == nil {
		t.Fatalf("an unreachable record minted a key anyway: %s", secretHex)
	}
	if secretHex != "" {
		t.Fatalf("a refusal still handed back a key: %s", secretHex)
	}
}

func TestCompanyPeopleEmailsReadsTheRecordAndNotAMessenger(t *testing.T) {
	service := serviceWhoseRecordAnswers(t, `{"people":[{"personID":"p-1","emails":["sample@example.com","second@example.com"]}]}`)

	emails, errorValue := service.companyPeopleEmails(context.Background())
	if errorValue != nil {
		t.Fatalf("company people emails: %v", errorValue)
	}
	if len(emails) != 2 || emails[0] != "sample@example.com" || emails[1] != "second@example.com" {
		t.Fatalf("the roster is not what the record holds: %v", emails)
	}
}

func TestCompanyPeopleEmailsRefusesAPartialRoster(t *testing.T) {
	service := serviceWhoseRecordIsDown(t)

	emails, errorValue := service.companyPeopleEmails(context.Background())
	if errorValue == nil {
		t.Fatalf("an unreachable record answered with a roster: %v", emails)
	}
}
