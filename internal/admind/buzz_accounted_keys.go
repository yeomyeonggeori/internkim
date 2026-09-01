package admind

import (
	"context"
	"errors"
	"fmt"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

var errDirectoryNamesNobody = errors.New("this device names nobody")

// Every key the company can account for: the two the device signs as, and every
// version of every address it knows. A key outside this set is one no address
// here derives, so nobody can be reached through it.
//
// The fleet directory has to answer for the set to mean anything. A roster
// assembled from caches alone is missing whoever the caches never saw, and a
// sweep that trusts it takes their seats away.
func (service *Service) accountedBuzzPubkeys(ctx context.Context, seed string) (map[string]bool, error) {
	if _, errorValue := service.currentUserRecords(ctx); errorValue != nil {
		return nil, fmt.Errorf("the directory did not answer: %w", errorValue)
	}
	accounted := map[string]bool{}
	for _, subject := range []string{buzzidentity.BootstrapSubject, buzzidentity.AgentSubject} {
		pubkey, errorValue := buzzPublicKey(buzzidentity.Secret(seed, subject))
		if errorValue != nil {
			return nil, errorValue
		}
		accounted[pubkey] = true
	}
	addresses := append(service.everyAddressThisDeviceKnows(ctx), service.invitedAddresses()...)
	if len(addresses) == 0 {
		return nil, errDirectoryNamesNobody
	}
	for _, email := range addresses {
		for _, pubkey := range service.everyKeyHeldBy(ctx, seed, email) {
			accounted[pubkey] = true
		}
	}
	return accounted, nil
}

// Somebody invited is somebody expected, and the directory does not carry them
// until they are added to it. Their seat is theirs in the meantime.
func (service *Service) invitedAddresses() []string {
	if !service.buzzInviteEnabled() {
		return nil
	}
	store := service.buzzStore()
	store.mutex.Lock()
	defer store.mutex.Unlock()
	addresses := []string{}
	for _, invite := range store.state.Invites {
		addresses = append(addresses, invite.Email)
	}
	for _, email := range store.state.Links {
		addresses = append(addresses, email)
	}
	return addresses
}
