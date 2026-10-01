package companyhost

import (
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

type Identity struct {
	AgentPrivateKey     string
	RelayOwnerPublicKey string
}

func IdentityForSeed(seed string) (Identity, error) {
	if seed == "" {
		return Identity{}, fmt.Errorf("the company identity seed is empty")
	}
	ownerPublicKey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
	if errorValue != nil {
		return Identity{}, errorValue
	}
	return Identity{
		AgentPrivateKey:     buzzidentity.Secret(seed, buzzidentity.AgentSubject),
		RelayOwnerPublicKey: ownerPublicKey,
	}, nil
}
