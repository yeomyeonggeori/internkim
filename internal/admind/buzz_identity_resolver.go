package admind

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	nostr "github.com/nbd-wtf/go-nostr"
	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

var errBuzzKeySeedMissing = errors.New("buzz key seed is not configured")

// mattermostBotBuzzEmail returns the Mattermost bot account's own email so that
// the bot's mirrored posts resolve to its Buzz bootstrap identity — the same key
// chatd signs the bot's own traffic with — instead of a per-email person key.
func (service *Service) mattermostBotBuzzEmail(ctx context.Context) string {
	service.botBuzzEmailOnce.Do(func() {
		token, errorValue := service.mattermostBotToken()
		if errorValue != nil {
			return
		}
		var userRecord mattermostUserRecord
		if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", token, nil, &userRecord); errorValue != nil {
			return
		}
		service.botBuzzEmailValue = strings.ToLower(strings.TrimSpace(userRecord.Email))
	})
	return service.botBuzzEmailValue
}

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
	if botEmail := service.mattermostBotBuzzEmail(ctx); botEmail != "" && strings.EqualFold(strings.TrimSpace(email), botEmail) {
		return buzzidentity.Secret(seed, buzzidentity.BootstrapSubject), nil
	}
	subject := service.buzzVaultSubject(ctx, email)
	version := service.buzzIdentityVersion(subject)
	secretHex := buzzidentity.Secret(seed, versionedSubject(email, version))
	if stored, errorValue := service.readBuzzIdentitySecret(subject); errorValue != nil || stored != secretHex {
		if errorValue := service.storeBuzzIdentitySecret(subject, secretHex); errorValue != nil {
			log.Printf("buzz identity vault pin failed for %s: %v", subject, errorValue)
		}
	}
	return secretHex, nil
}

// buzzVaultSubject keys the vault by personID when the person is known to the
// policy, falling back to the normalized email before the person is provisioned.
func (service *Service) buzzVaultSubject(ctx context.Context, email string) string {
	personID, errorValue := service.localBlueclawPersonIDByEmail(ctx, email)
	if errorValue == nil && strings.TrimSpace(personID) != "" {
		return strings.TrimSpace(personID)
	}
	return normalizedVaultSubject(email)
}

func buzzPublicKey(secretHex string) (string, error) {
	return nostr.GetPublicKey(secretHex)
}
