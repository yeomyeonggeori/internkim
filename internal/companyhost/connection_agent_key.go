package companyhost

import (
	"encoding/json"

	"github.com/yeomyeonggeori/internkim/internal/durablefile"
)

const connectionFileMode = 0o600

func KeepAgentKey(connectionPath, agentKey string) error {
	connection, errorValue := ReadConnection(connectionPath)
	if errorValue != nil {
		return errorValue
	}
	connection.AgentKey = agentKey
	if errorValue := validateConnection(connection); errorValue != nil {
		return errorValue
	}
	return writeConnection(connectionPath, connection)
}

func writeConnection(connectionPath string, connection Connection) error {
	document, errorValue := json.MarshalIndent(connection, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return durablefile.Write(connectionPath, append(document, '\n'), connectionFileMode)
}
