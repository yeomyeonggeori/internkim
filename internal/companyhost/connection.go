package companyhost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const connectionSchemaVersion = 1

var (
	secretPattern      = regexp.MustCompile(`^[a-f0-9]{64}$`)
	hostSessionPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)
)

type Company struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type CentralPlane struct {
	ProjectURL     string `json:"projectURL"`
	PublishableKey string `json:"publishableKey"`
}

type Connection struct {
	SchemaVersion int          `json:"schemaVersion"`
	AppURL        string       `json:"appURL"`
	Company       Company      `json:"company"`
	CentralPlane  CentralPlane `json:"centralPlane"`
	GatewayURL    string       `json:"gatewayURL"`
	AgentKey      string       `json:"agentKey"`
}

func ReadConnection(path string) (Connection, error) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return Connection{}, errorValue
	}
	return ParseConnection(document)
}

func ParseConnection(document []byte) (Connection, error) {
	var connection Connection
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&connection); errorValue != nil {
		return Connection{}, fmt.Errorf("the connection file is not the one company setup issues: %w", errorValue)
	}
	return validatedConnection(connection)
}

func validateConnection(connection Connection) error {
	if connection.SchemaVersion != connectionSchemaVersion {
		return fmt.Errorf("this installer needs a version %d connection file from company setup", connectionSchemaVersion)
	}
	for _, field := range []struct{ name, value string }{
		{"company.id", connection.Company.ID},
		{"company.name", connection.Company.Name},
		{"company.slug", connection.Company.Slug},
		{"centralPlane.publishableKey", connection.CentralPlane.PublishableKey},
	} {
		if errorValue := validateTextField(field.name, field.value); errorValue != nil {
			return errorValue
		}
	}
	if _, errorValue := uuid.Parse(connection.Company.ID); errorValue != nil {
		return fmt.Errorf("the connection file has no valid company.id. Download it again from company setup")
	}
	for _, address := range []struct {
		name      string
		value     string
		protocols []string
	}{
		{"appURL", connection.AppURL, []string{"https", "http"}},
		{"centralPlane.projectURL", connection.CentralPlane.ProjectURL, []string{"https", "http"}},
		{"gatewayURL", connection.GatewayURL, []string{"wss", "ws"}},
	} {
		if errorValue := validateAddress(address.name, address.value, address.protocols); errorValue != nil {
			return errorValue
		}
	}
	if !secretPattern.MatchString(connection.AgentKey) && !hostSessionPattern.MatchString(connection.AgentKey) {
		return fmt.Errorf("the connection file contains an invalid company key")
	}
	return nil
}

func validateTextField(name, value string) error {
	if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("the connection file has no valid %s. Download it again from company setup", name)
	}
	return nil
}

func validateAddress(name, address string, protocols []string) error {
	if errorValue := validateTextField(name, address); errorValue != nil {
		return errorValue
	}
	parsed, errorValue := url.Parse(address)
	if errorValue != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("the connection file contains an invalid service address in %s", name)
	}
	for _, protocol := range protocols {
		if parsed.Scheme == protocol {
			return nil
		}
	}
	return fmt.Errorf("the connection file contains an invalid service address in %s", name)
}

func normalizeConnection(connection Connection) Connection {
	connection.AppURL = strings.TrimRight(connection.AppURL, "/")
	connection.CentralPlane.ProjectURL = strings.TrimRight(connection.CentralPlane.ProjectURL, "/")
	connection.GatewayURL = strings.TrimRight(connection.GatewayURL, "/")
	return connection
}
