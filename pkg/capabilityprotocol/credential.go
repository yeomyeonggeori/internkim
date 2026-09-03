package capabilityprotocol

var messengerIdentityCredentialKinds = mustReadGeneratedSchemaEnum("messenger-identity-credential-kind")

func MessengerIdentityCredentialKind() string {
	if len(messengerIdentityCredentialKinds) != 1 {
		panic("the generated messenger identity credential kinds name more than one kind")
	}
	return messengerIdentityCredentialKinds[0]
}
