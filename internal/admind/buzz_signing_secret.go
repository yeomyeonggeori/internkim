package admind

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

var errBuzzSigningSecretUnknownKey = errors.New("no member of this company currently signs with that key")

type buzzSigningSecretRequest struct {
	PubkeyHex string `json:"pubkeyHex"`
}

type buzzSigningSecretResponse struct {
	SecretHex string `json:"secretHex"`
}

func (service *Service) handleBuzzSigningSecret(responseWriter http.ResponseWriter, request *http.Request) {
	var requestDocument buzzSigningSecretRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil {
		http.Error(responseWriter, "this action names the key it is asking about", http.StatusBadRequest)
		return
	}
	pubkeyHex := strings.ToLower(strings.TrimSpace(requestDocument.PubkeyHex))
	if !isBuzzPubkeyHex(pubkeyHex) {
		http.Error(responseWriter, "pubkeyHex must be 64 hexadecimal characters", http.StatusBadRequest)
		return
	}
	secretHex, errorValue := service.buzzSigningSecretForPubkey(request.Context(), pubkeyHex)
	switch {
	case errors.Is(errorValue, errBuzzKeySeedMissing):
		http.Error(responseWriter, "this device names no buzz key seed", http.StatusNotImplemented)
	case errors.Is(errorValue, errBuzzSigningSecretUnknownKey):
		http.Error(responseWriter, errBuzzSigningSecretUnknownKey.Error(), http.StatusNotFound)
	case errorValue != nil:
		http.Error(responseWriter, "buzz_signing_secret_failed", http.StatusBadGateway)
	default:
		service.writeJSON(responseWriter, buzzSigningSecretResponse{SecretHex: secretHex})
	}
}

func (service *Service) buzzSigningSecretForPubkey(ctx context.Context, pubkeyHex string) (string, error) {
	seed := service.buzzKeySeed()
	if seed == "" {
		return "", errBuzzKeySeedMissing
	}
	personIDs, errorValue := service.localBlueclawPersonIDsByEmail(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	for _, email := range service.everyAddressThisDeviceKnows(ctx) {
		secretHex := service.currentBuzzSecret(seed, email, vaultSubjectForPerson(personIDs[email], email))
		derived, errorValue := buzzPublicKey(secretHex)
		if errorValue != nil {
			return "", errorValue
		}
		if derived == pubkeyHex {
			return secretHex, nil
		}
	}
	return "", errBuzzSigningSecretUnknownKey
}

func isBuzzPubkeyHex(value string) bool {
	if len(value) != buzzIdentitySecretHexLength {
		return false
	}
	_, errorValue := hex.DecodeString(value)
	return errorValue == nil
}
