package admind

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const buzzClientVaultDirectoryName = "buzz-client-vault"
const buzzClientVaultMaxBytes = 8 * 1024

// The client vault holds each person's Buzz secret key already sealed by their
// own password or passkey in the browser. admind stores it as an opaque blob it
// cannot open, so a server or device compromise never yields a usable key.
func (service *Service) buzzClientVaultPath(subject string) string {
	return filepath.Join(service.Configuration.StateDirectory, buzzClientVaultDirectoryName, subject+".json")
}

func isSafeVaultSubject(subject string) bool {
	if subject == "" {
		return false
	}
	for _, character := range subject {
		isLower := character >= 'a' && character <= 'z'
		isDigit := character >= '0' && character <= '9'
		isAllowedSymbol := character == '-' || character == '_' || character == '.' || character == '@'
		if !isLower && !isDigit && !isAllowedSymbol {
			return false
		}
	}
	return true
}

func (service *Service) storeBuzzClientVault(subject string, blob []byte) error {
	path := service.buzzClientVaultPath(subject)
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(path, blob, 0o600)
}

func (service *Service) readBuzzClientVault(subject string) ([]byte, bool, error) {
	blob, errorValue := os.ReadFile(service.buzzClientVaultPath(subject))
	if errors.Is(errorValue, os.ErrNotExist) {
		return nil, false, nil
	}
	if errorValue != nil {
		return nil, false, errorValue
	}
	return blob, true, nil
}

func (service *Service) handleBuzzClientVault(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := service.webActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	subject := strings.TrimSpace(service.buzzVaultSubject(request.Context(), actorEmail))
	if !isSafeVaultSubject(subject) {
		http.Error(responseWriter, "invalid identity", http.StatusBadRequest)
		return
	}
	switch request.Method {
	case http.MethodGet:
		service.writeBuzzClientVault(responseWriter, subject)
	case http.MethodPost:
		service.acceptBuzzClientVault(responseWriter, request, subject)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) writeBuzzClientVault(responseWriter http.ResponseWriter, subject string) {
	blob, found, errorValue := service.readBuzzClientVault(subject)
	if errorValue != nil {
		http.Error(responseWriter, "vault_read_failed", http.StatusInternalServerError)
		return
	}
	if !found {
		service.writeJSON(responseWriter, map[string]bool{"found": false})
		return
	}
	service.writeJSON(responseWriter, map[string]any{"found": true, "wrapped": json.RawMessage(blob)})
}

func (service *Service) acceptBuzzClientVault(responseWriter http.ResponseWriter, request *http.Request, subject string) {
	body, errorValue := io.ReadAll(io.LimitReader(request.Body, buzzClientVaultMaxBytes))
	if errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	if !json.Valid(body) {
		http.Error(responseWriter, "wrapped secret must be JSON", http.StatusBadRequest)
		return
	}
	if errorValue := service.storeBuzzClientVault(subject, body); errorValue != nil {
		http.Error(responseWriter, "vault_write_failed", http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"stored": true})
}
