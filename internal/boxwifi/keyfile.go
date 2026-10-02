package boxwifi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultKeyfileDirectory = "/etc/NetworkManager/system-connections"
	keyfileExtension        = ".nmconnection"
	keyfileMode             = 0o600
)

var keyfileValueEscaper = strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`)

func (radio NetworkManagerRadio) keyfileDirectory() string {
	if radio.KeyfileDirectory != "" {
		return radio.KeyfileDirectory
	}
	return defaultKeyfileDirectory
}

func (radio NetworkManagerRadio) keyfilePath(fileName string) string {
	fileName = strings.ReplaceAll(fileName, "/", "_") + keyfileExtension
	return filepath.Join(radio.keyfileDirectory(), fileName)
}

func (radio NetworkManagerRadio) loadProfile(ctx context.Context, fileName, connectionName, ssid, password string) ([]byte, error) {
	path, errorValue := radio.writeKeyfile(fileName, connectionName, ssid, password)
	if errorValue != nil {
		return nil, errorValue
	}
	output, errorValue := radio.run(ctx, "connection", "load", path)
	if errorValue != nil {
		radio.removeKeyfile(fileName)
	}
	return output, errorValue
}

func (radio NetworkManagerRadio) writeKeyfile(fileName, connectionName, ssid, password string) (string, error) {
	if strings.ContainsRune(ssid, 0) || strings.ContainsRune(password, 0) {
		return "", errors.New("a network name or password cannot contain a NUL byte")
	}
	directory := radio.keyfileDirectory()
	if errorValue := os.MkdirAll(directory, 0o700); errorValue != nil {
		return "", fmt.Errorf("preparing %s: %w", directory, errorValue)
	}
	path := radio.keyfilePath(fileName)
	if errorValue := os.Remove(path); errorValue != nil && !os.IsNotExist(errorValue) {
		return "", fmt.Errorf("replacing %s: %w", path, errorValue)
	}
	file, errorValue := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, keyfileMode)
	if errorValue != nil {
		return "", fmt.Errorf("creating %s: %w", path, errorValue)
	}
	if _, errorValue := file.WriteString(renderKeyfile(connectionName, ssid, password)); errorValue != nil {
		file.Close()
		os.Remove(path)
		return "", fmt.Errorf("writing %s: %w", path, errorValue)
	}
	if errorValue := file.Close(); errorValue != nil {
		os.Remove(path)
		return "", fmt.Errorf("writing %s: %w", path, errorValue)
	}
	return path, nil
}

func (radio NetworkManagerRadio) removeKeyfile(fileName string) {
	os.Remove(radio.keyfilePath(fileName))
}

func renderKeyfile(connectionName, ssid, password string) string {
	var keyfile strings.Builder
	keyfile.WriteString("[connection]\nid=" + escapeKeyfileValue(connectionName) + "\ntype=wifi\nautoconnect=true\n\n")
	keyfile.WriteString("[wifi]\nmode=infrastructure\nhidden=true\nssid=" + escapeKeyfileValue(strings.ReplaceAll(ssid, ";", `\;`)) + "\n\n")
	if password != "" {
		keyfile.WriteString("[wifi-security]\nkey-mgmt=wpa-psk\npsk=" + escapeKeyfileValue(password) + "\n\n")
	}
	keyfile.WriteString("[ipv4]\nmethod=auto\n\n[ipv6]\nmethod=auto\n")
	return keyfile.String()
}

func escapeKeyfileValue(value string) string {
	escaped := keyfileValueEscaper.Replace(value)
	if strings.HasPrefix(escaped, " ") {
		escaped = `\s` + escaped[1:]
	}
	if strings.HasSuffix(escaped, " ") {
		escaped = escaped[:len(escaped)-1] + `\s`
	}
	return escaped
}
