package capabilityprotocol

import (
	"encoding/json"
	"slices"
)

var connectorPlatformNames = mustReadPlatformNames("connector-platform")

var messengerPlatformNames = mustReadPlatformNames("messenger-platform")

func ConnectorPlatformNames() []string {
	return slices.Clone(connectorPlatformNames)
}

func MessengerPlatformNames() []string {
	return slices.Clone(messengerPlatformNames)
}

func IsConnectorPlatform(name string) bool {
	return slices.Contains(connectorPlatformNames, name)
}

func IsMessengerPlatform(name string) bool {
	return slices.Contains(messengerPlatformNames, name)
}

func mustReadPlatformNames(schemaName string) []string {
	document, errorValue := generatedCatalogFiles.ReadFile("generated/json-schema/" + schemaName + ".schema.json")
	if errorValue != nil {
		panic(errorValue)
	}
	schema := struct {
		Enum []string `json:"enum"`
	}{}
	if errorValue := json.Unmarshal(document, &schema); errorValue != nil {
		panic(errorValue)
	}
	if len(schema.Enum) == 0 {
		panic("the generated " + schemaName + " schema names no platform")
	}
	return schema.Enum
}
