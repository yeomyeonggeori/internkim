package admind

import (
	"context"
	"database/sql"
	"log"
	"sort"
)

// A room the company opened holds the company. A seat in one that no address
// here derives was left behind by an address the directory has since dropped,
// and the person it once named is not reachable through it, so the room takes
// it back. Membership is re-addable, which is why this sweeps rather than asks.
func (service *Service) removeSeatsNobodyAccountsFor(ctx context.Context, relay *sql.DB, channelIDs []string, seed string) {
	accounted, errorValue := service.accountedBuzzPubkeys(ctx, seed)
	if errorValue != nil {
		log.Printf("buzz membership: no seat is swept, %v", errorValue)
		return
	}
	for _, channelID := range channelIDs {
		heldRoles, errorValue := buzzChannelMemberRoles(ctx, relay, channelID)
		if errorValue != nil {
			log.Printf("buzz membership: reading who is in %s failed: %v", channelID, errorValue)
			continue
		}
		strangers := seatsNobodyAccountsFor(heldRoles, accounted)
		if len(strangers) == 0 {
			continue
		}
		removed, errorValue := removeBuzzChannelMembers(ctx, relay, channelID, strangers)
		if errorValue != nil {
			log.Printf("buzz membership: %s keeps %d seat(s) nobody accounts for: %v", channelID, len(strangers), errorValue)
			continue
		}
		log.Printf("buzz membership: %s took back %d seat(s) nobody accounts for: %v", channelID, removed, strangers)
		if errorValue := service.tellClientsWhoIsInTheRoom(ctx, relay, channelID); errorValue != nil {
			log.Printf("buzz membership: %s changed and no client was told: %v", channelID, errorValue)
		}
	}
}

func seatsNobodyAccountsFor(heldRoles map[string]string, accounted map[string]bool) []string {
	strangers := []string{}
	for pubkey := range heldRoles {
		if !accounted[pubkey] {
			strangers = append(strangers, pubkey)
		}
	}
	sort.Strings(strangers)
	return strangers
}
