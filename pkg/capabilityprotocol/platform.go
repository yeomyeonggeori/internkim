package capabilityprotocol

import "slices"

var connectorPlatformNames = mustReadGeneratedSchemaEnum("connector-platform")

var messengerPlatformNames = mustReadGeneratedSchemaEnum("messenger-platform")

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
