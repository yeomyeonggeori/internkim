package admind

import (
	"errors"
	"net/http"
)

type buzzClaimResponse struct {
	SecretHex string `json:"secretHex"`
	PublicHex string `json:"publicHex"`
}

// handleBuzzClaim hands the caller their own Buzz secret key once, gated on the
// Cloudflare Access verified email. The browser wraps it with the user's
// passkey or password and stores only the sealed blob (POST /agent/api/buzz-
// vault); the raw key never persists server-side beyond the derivation seed.
// The gate is Cloudflare Access — an authentication layer, not a messaging
// platform — so the Buzz identity stays independent of Mattermost or any client.
func (service *Service) handleBuzzClaim(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	actorEmail := service.authenticatedCallerEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "Cloudflare Access authentication is required", http.StatusUnauthorized)
		return
	}
	secretHex, errorValue := service.personBuzzSecret(request.Context(), actorEmail)
	if errors.Is(errorValue, errBuzzKeySeedMissing) {
		http.Error(responseWriter, "buzz key seed is not configured", http.StatusNotImplemented)
		return
	}
	if errorValue != nil {
		http.Error(responseWriter, "buzz_claim_failed", http.StatusInternalServerError)
		return
	}
	publicHex, errorValue := buzzPublicKey(secretHex)
	if errorValue != nil {
		http.Error(responseWriter, "buzz_claim_failed", http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, buzzClaimResponse{SecretHex: secretHex, PublicHex: publicHex})
}
