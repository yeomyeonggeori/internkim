package llmbackend

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const FailureEvidenceMarker = "llm failure evidence: /admin/api/diagnostics/llm-failure?id="

func FailureEvidenceDirectory(workspacePath string) string {
	return filepath.Join(workspacePath, ".blueclaw", "llm-failures")
}

func WriteFailureEvidence(workspacePath string, request any, capture *FailureCapture) (string, error) {
	directoryPath := FailureEvidenceDirectory(workspacePath)
	if errorValue := os.MkdirAll(directoryPath, 0700); errorValue != nil {
		return "", errorValue
	}
	identifierBytes := make([]byte, 16)
	if _, errorValue := rand.Read(identifierBytes); errorValue != nil {
		return "", errorValue
	}
	identifier := hex.EncodeToString(identifierBytes)
	document, errorValue := json.Marshal(struct {
		CreatedAt time.Time              `json:"createdAt"`
		Request   any                    `json:"request"`
		Exchanges FailureCaptureSnapshot `json:"exchanges"`
	}{time.Now().UTC(), request, capture.Snapshot()})
	if errorValue != nil {
		return "", errorValue
	}
	return identifier, writeFailureEvidenceDocument(directoryPath, identifier, document)
}

func writeFailureEvidenceDocument(directoryPath, identifier string, document []byte) error {
	file, errorValue := os.CreateTemp(directoryPath, ".pending-*")
	if errorValue != nil {
		return errorValue
	}
	defer os.Remove(file.Name())
	_, writeError := file.Write(document)
	if writeError == nil {
		writeError = file.Sync()
	}
	closeError := file.Close()
	if writeError != nil {
		return writeError
	}
	if closeError != nil {
		return closeError
	}
	return os.Rename(file.Name(), filepath.Join(directoryPath, identifier+".json"))
}

func ReadFailureEvidence(workspacePath, identifier string) ([]byte, error) {
	decoded, errorValue := hex.DecodeString(identifier)
	if errorValue != nil || len(decoded) != 16 || hex.EncodeToString(decoded) != identifier {
		return nil, fmt.Errorf("invalid llm failure evidence identifier")
	}
	return os.ReadFile(filepath.Join(FailureEvidenceDirectory(workspacePath), identifier+".json"))
}
