package admind

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	nostr "github.com/nbd-wtf/go-nostr"
	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

var errBuzzKeySeedMissing = errors.New("buzz key seed is not configured")

func (service *Service) buzzKeySeed() string {
	service.buzzKeySeedOnce.Do(func() {
		path := strings.TrimSpace(service.Configuration.BuzzKeySeedPath)
		if path == "" {
			return
		}
		contents, errorValue := os.ReadFile(path)
		if errorValue != nil {
			return
		}
		service.buzzKeySeedValue = strings.TrimSpace(string(contents))
	})
	return service.buzzKeySeedValue
}

// personBuzzSecret resolves the caller's stable Buzz secret. The secret is a
// deterministic function of the person's email (matching the history importer,
// so imported history and live traffic share one identity), but it is pinned in
// the vault under the person's personID. Pinning by personID means a later email
// change keeps the same Buzz identity and message history.
func (service *Service) personBuzzSecret(ctx context.Context, email string) (string, error) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return "", errBuzzKeySeedMissing
	}
	vaultSubject, errorValue := service.buzzVaultSubject(ctx, email)
	if errorValue != nil {
		return "", errorValue
	}
	return service.currentBuzzSecret(seed, email, vaultSubject), nil
}

func (service *Service) currentBuzzSecret(seed string, email string, vaultSubject string) string {
	secretHex := buzzidentity.Secret(seed, versionedSubject(email, service.buzzIdentityVersion(vaultSubject)))
	service.pinBuzzIdentitySecret(vaultSubject, secretHex)
	return secretHex
}

func (service *Service) pinBuzzIdentitySecret(vaultSubject string, secretHex string) {
	if stored, errorValue := service.readBuzzIdentitySecret(vaultSubject); errorValue == nil && stored == secretHex {
		return
	}
	if errorValue := service.storeBuzzIdentitySecret(vaultSubject, secretHex); errorValue != nil {
		log.Printf("buzz identity vault pin failed for %s: %v", vaultSubject, errorValue)
	}
}

// buzzVaultSubject keys the vault by personID when the person is known to the
// record, falling back to the normalized email before the person is provisioned.
func (service *Service) buzzVaultSubject(ctx context.Context, email string) (string, error) {
	personID, errorValue := service.localBlueclawPersonIDByEmail(ctx, email)
	if errorValue != nil {
		return "", fmt.Errorf("the record did not say who %s is: %w", normalizedVaultSubject(email), errorValue)
	}
	return vaultSubjectForPerson(personID, email), nil
}

func vaultSubjectForPerson(personID string, email string) string {
	if trimmedPersonID := strings.TrimSpace(personID); trimmedPersonID != "" {
		return trimmedPersonID
	}
	return normalizedVaultSubject(email)
}

func buzzPublicKey(secretHex string) (string, error) {
	return nostr.GetPublicKey(secretHex)
}
