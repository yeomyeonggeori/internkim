package admind

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

var errDirectoryNamesNobody = errors.New("this device names nobody")

// Every key the company can account for: the two the device signs as, and every
// version of every address the directory holds. A key outside this set is one
// nobody here derives, so nobody can be reached through it.
func (service *Service) accountedBuzzPubkeys(ctx context.Context, seed string) (map[string]bool, error) {
	addresses, errorValue := service.addressesTheDirectoryHolds(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	if len(addresses) == 0 {
		return nil, errDirectoryNamesNobody
	}
	accounted := map[string]bool{}
	for _, subject := range []string{buzzidentity.BootstrapSubject, buzzidentity.AgentSubject} {
		pubkey, errorValue := buzzPublicKey(buzzidentity.Secret(seed, subject))
		if errorValue != nil {
			return nil, errorValue
		}
		accounted[pubkey] = true
	}
	for _, email := range addresses {
		held, errorValue := service.everyKeyHeldBy(ctx, seed, email)
		if errorValue != nil {
			return nil, errorValue
		}
		for _, pubkey := range held {
			accounted[pubkey] = true
		}
	}
	return accounted, nil
}

// Who works here, asked of the directory itself rather than of anything that
// remembers what it once said. The caches admind seats people from are there so
// a room still fills during an outage; a cache that has not caught up with
// somebody leaving is exactly what leaves their seat behind, so no cache
// answers this.
func (service *Service) addressesTheDirectoryHolds(ctx context.Context) ([]string, error) {
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return nil, fmt.Errorf("the directory did not answer: %w", errorValue)
	}
	addresses := []string{}
	for _, record := range records {
		addresses = append(addresses, record.Email)
	}
	if client := service.centralPlane(); client != nil {
		members, errorValue := client.Members(ctx)
		if errorValue != nil {
			return nil, fmt.Errorf("the company did not answer with its members: %w", errorValue)
		}
		for _, member := range members {
			if member.IsActive() {
				addresses = append(addresses, member.Email)
			}
		}
	}
	addresses = append(addresses, service.addressesInvitedAndStillWaiting()...)
	return append(addresses, service.seedAdminEmail(), service.claimedAdminEmail()), nil
}

// Somebody invited is expected, and the directory does not carry them until
// they are added to it. An invitation that has run out is not somebody expected.
func (service *Service) addressesInvitedAndStillWaiting() []string {
	if !service.buzzInviteEnabled() {
		return nil
	}
	store := service.buzzStore()
	store.mutex.Lock()
	defer store.mutex.Unlock()
	now := time.Now().UTC()
	addresses := []string{}
	for _, invite := range store.state.Invites {
		if now.Before(invite.ExpiresAt) {
			addresses = append(addresses, invite.Email)
		}
	}
	return addresses
}
