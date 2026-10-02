package companyhost

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	connectionFileName   = "connection.json"
	secretDirectoryName  = "secrets"
	agentKeyFileName     = "agent-key"
	modelKeyFileName     = "openrouter-key"
	identitySeedFileName = "buzz-key-seed"
)

func writePrivateFile(path string, content []byte) error {
	file, errorValue := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	if errorValue := file.Chmod(0o600); errorValue != nil {
		return errorValue
	}
	_, errorValue = file.Write(content)
	return errorValue
}

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
	if errorValue := writePrivateFile(filepath.Join(directoryPath, connectionFileName), append(document, '\n')); errorValue != nil {
		return "", errorValue
	}
	if errorValue := writePrivateFile(filepath.Join(secretDirectoryPath, agentKeyFileName), []byte(connection.AgentKey+"\n")); errorValue != nil {
		return "", errorValue
	}
	return secretDirectoryPath, nil
}

func KeepSecret(secretDirectoryPath, name string) (string, error) {
	path := filepath.Join(secretDirectoryPath, name)
	existing, errorValue := os.ReadFile(path)
	if errorValue == nil {
		if chmodError := os.Chmod(path, 0o600); chmodError != nil {
			return "", chmodError
		}
		value := strings.TrimSpace(string(existing))
		if !secretPattern.MatchString(value) {
			return "", fmt.Errorf("saved %s is invalid. Restore it from backup; it will not be replaced", name)
		}
		return value, nil
	}
	if !os.IsNotExist(errorValue) {
		return "", errorValue
	}
	generated := make([]byte, 32)
	if _, errorValue := rand.Read(generated); errorValue != nil {
		return "", errorValue
	}
	value := hex.EncodeToString(generated)
	if errorValue := writePrivateFile(path, []byte(value+"\n")); errorValue != nil {
		return "", errorValue
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
	return writeModelKey(path, value)
}

func writeModelKey(path, value string) error {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("enter the OpenRouter API key from your account's Keys page")
	}
	return writePrivateFile(path, []byte(value+"\n"))
}
