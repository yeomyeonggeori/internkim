package admind

import (
	"context"
	"log"
	"net/http"
)

func (service *Service) buzzSecretForEmail(ctx context.Context, email string) string {
	seed := service.buzzKeySeed()
	if seed == "" {
		return ""
	}
	subject, errorValue := service.buzzVaultSubject(ctx, email)
	if errorValue != nil {
		log.Printf("buzz identity withheld: %v", errorValue)
		return ""
	}
	version := service.buzzIdentityVersion(subject)
	return buzzKeyForVersion(seed, email, version)
}

// handleAuthIdentity hands an already-signed-in session its own deterministic
// Buzz key so the browser can establish the messaging identity without a
// separate vault-setup dialog. The session is the gate; the key is a function
// of the session email, so this exposes nothing the account does not already own.
func (service *Service) handleAuthIdentity(responseWriter http.ResponseWriter, request *http.Request) {
	email := service.webActorEmail(request)
	if email == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	secretHex := service.buzzSecretForEmail(request.Context(), email)
	if secretHex == "" {
		http.Error(responseWriter, "buzz identity unavailable", http.StatusNotImplemented)
		return
	}
	service.writeJSON(responseWriter, map[string]string{"secretHex": secretHex})
}
