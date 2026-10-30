package companyhost

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/durablefile"
)

const (
	connectionFileName   = "connection.json"
	secretDirectoryName  = "secrets"
	agentKeyFileName     = "agent-key"
	modelKeyFileName     = "openrouter-key"
	identitySeedFileName = "buzz-key-seed"
)

func PrepareStateDirectory(directoryPath string, connection Connection) (string, error) {
	if errorValue := os.MkdirAll(directoryPath, 0o700); errorValue != nil {
		return "", errorValue
	}
	if errorValue := os.Chmod(directoryPath, 0o700); errorValue != nil {
		return "", errorValue
	}
	secretDirectoryPath := filepath.Join(directoryPath, secretDirectoryName)
	if errorValue := os.MkdirAll(secretDirectoryPath, 0o700); errorValue != nil {
		return "", errorValue
	}
	if errorValue := os.Chmod(secretDirectoryPath, 0o700); errorValue != nil {
		return "", errorValue
	}
	document, errorValue := json.MarshalIndent(connection, "", "  ")
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := durablefile.Write(filepath.Join(directoryPath, connectionFileName), append(document, '\n'), 0o600); errorValue != nil {
		return "", errorValue
	}
	if errorValue := durablefile.Write(filepath.Join(secretDirectoryPath, agentKeyFileName), []byte(connection.AgentKey+"\n"), 0o600); errorValue != nil {
		return "", errorValue
	}
	return secretDirectoryPath, nil
}

func KeepSecret(secretDirectoryPath, name string) (string, error) {
	path := filepath.Join(secretDirectoryPath, name)
	existing, errorValue := os.ReadFile(path)
	if errorValue != nil && !os.IsNotExist(errorValue) {
		return "", errorValue
	}
	if errorValue == nil && strings.TrimSpace(string(existing)) != "" {
		return savedSecret(path, name, existing)
	}
	generated := make([]byte, 32)
	if _, errorValue := rand.Read(generated); errorValue != nil {
		return "", errorValue
	}
	value := hex.EncodeToString(generated)
	if errorValue := durablefile.Write(path, []byte(value+"\n"), 0o600); errorValue != nil {
		return "", errorValue
	}
	return value, nil
}

func savedSecret(path, name string, existing []byte) (string, error) {
	if errorValue := os.Chmod(path, 0o600); errorValue != nil {
		return "", errorValue
	}
	value := strings.TrimSpace(string(existing))
	if !secretPattern.MatchString(value) {
		return "", fmt.Errorf("saved %s is invalid. Restore it from backup; it will not be replaced", name)
	}
	return value, nil
}

func KeepModelKey(secretDirectoryPath, suppliedKey string, promptForKey func() (string, error)) error {
	path := filepath.Join(secretDirectoryPath, modelKeyFileName)
	if suppliedKey != "" {
		return writeModelKey(path, suppliedKey)
	}
	existing, errorValue := os.ReadFile(path)
	if errorValue != nil && !os.IsNotExist(errorValue) {
		return errorValue
	}
	if errorValue == nil {
		if strings.TrimSpace(string(existing)) == "" {
			return fmt.Errorf("the saved OpenRouter key is empty. Supply --model-key-file to replace it")
		}
		return os.Chmod(path, 0o600)
	}
	value, errorValue := promptForKey()
	if errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return writeModelKey(path, value)
}

func writeModelKey(path, value string) error {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("enter the OpenRouter API key from your account's Keys page")
	}
	return durablefile.Write(path, []byte(value+"\n"), 0o600)
}
