package capabilityprotocol

import "slices"

var messengerPlatformNames = mustReadGeneratedSchemaEnum("messenger-platform")

func IsMessengerPlatform(name string) bool {
	return slices.Contains(messengerPlatformNames, name)
}
