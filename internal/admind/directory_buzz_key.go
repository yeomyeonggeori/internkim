package admind

import (
	"encoding/json"
	"net/http"
	"strings"
)

type directoryBuzzKeyRequest struct {
	Email string `json:"email"`
}

type directoryBuzzKeyResponse struct {
	PubkeyHex string `json:"pubkeyHex"`
}

// handleDirectoryBuzzKey answers a member's current Buzz public key so the
// capability layer can address a direct message to them. Only the public half
// leaves this process; the secret stays with the seed. A non-member gets no
// key, because deriving one would mint an identity for someone who is not here.
func (service *Service) handleDirectoryBuzzKey(responseWriter http.ResponseWriter, request *http.Request) {
	var payload directoryBuzzKeyRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(payload.Email))
	if normalizedEmail == "" {
		http.Error(responseWriter, "email is required", http.StatusBadRequest)
		return
	}
	client := service.centralPlane()
	if client == nil {
		http.Error(responseWriter, "this host has no company directory configured", http.StatusBadGateway)
		return
	}
	member, isKnown, errorValue := client.MemberByEmail(request.Context(), normalizedEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if !isKnown || !member.IsActive() {
		http.Error(responseWriter, "no active member has this address", http.StatusNotFound)
		return
	}
	secretHex, errorValue := service.personBuzzSecret(request.Context(), normalizedEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	pubkeyHex, errorValue := buzzPublicKey(secretHex)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(directoryBuzzKeyResponse{PubkeyHex: pubkeyHex})
}
