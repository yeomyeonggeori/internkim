package admind

import (
	"errors"
	"io"
	"net/http"
	"os"
)

const googleOAuthClientUploadMaxBytes = 1 << 20

type googleOAuthClientUploadResponse struct {
	Configured bool   `json:"configured"`
	ClientID   string `json:"clientID"`
}

func (service *Service) uploadGoogleOAuthClient(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.canManageGoogleOAuth(request) {
		http.Error(responseWriter, "admin access required", http.StatusForbidden)
		return
	}
	clientDocument, errorValue := readGoogleOAuthClientUpload(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	clientID, errorValue := validateGoogleOAuthClientDocument(clientDocument)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.writeGoogleOAuthClientDocument(clientDocument); errorValue != nil {
		http.Error(responseWriter, "failed to store google oauth client", http.StatusInternalServerError)
		return
	}
	if errorValue := service.resetGoogleOAuthAccountConnection(request.Context()); errorValue != nil {
		http.Error(responseWriter, "failed to reset google calendar connection", http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, googleOAuthClientUploadResponse{
		Configured: true,
		ClientID:   clientID,
	})
}

func readGoogleOAuthClientUpload(request *http.Request) ([]byte, error) {
	request.Body = http.MaxBytesReader(nil, request.Body, googleOAuthClientUploadMaxBytes)
	file, _, errorValue := request.FormFile("client")
	if errorValue != nil {
		if isGoogleOAuthClientUploadTooLarge(errorValue) {
			return nil, errors.New("client.json file is too large")
		}
		return nil, errors.New("client.json file is required")
	}
	defer file.Close()
	document, errorValue := io.ReadAll(file)
	if errorValue != nil {
		if isGoogleOAuthClientUploadTooLarge(errorValue) {
			return nil, errors.New("client.json file is too large")
		}
		return nil, errorValue
	}
	if len(document) == 0 {
		return nil, errors.New("client.json file is empty")
	}
	return document, nil
}

func isGoogleOAuthClientUploadTooLarge(errorValue error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(errorValue, &maxBytesError)
}

func validateGoogleOAuthClientDocument(document []byte) (string, error) {
	credentials, errorValue := parseGoogleOAuthClientDocument(document)
	if errorValue != nil {
		return "", errorValue
	}
	return credentials.ClientID, nil
}

func (service *Service) writeGoogleOAuthClientDocument(document []byte) error {
	directory := service.calendarSecretsDirectory()
	if errorValue := os.MkdirAll(directory, 0o700); errorValue != nil {
		return errorValue
	}
	if errorValue := os.Chmod(directory, 0o700); errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(service.googleOAuthClientFilePath(), document, 0o600)
}
