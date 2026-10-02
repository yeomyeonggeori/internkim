package companyhost

import (
	"github.com/nbd-wtf/go-nostr"
	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

type Identity struct {
	AgentPrivateKey     string
	RelayOwnerPublicKey string
}

func IdentityForSeed(seed string) (Identity, error) {
	ownerPublicKey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
	if errorValue != nil {
		return Identity{}, errorValue
	}
	return Identity{
		AgentPrivateKey:     buzzidentity.Secret(seed, buzzidentity.AgentSubject),
		RelayOwnerPublicKey: ownerPublicKey,
	}, nil
}
