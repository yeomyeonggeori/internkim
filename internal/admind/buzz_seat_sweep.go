package admind

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"sort"
	"strings"
)

// A room the company opened holds the company. A seat in one that no address
// here derives was left behind by an address the directory has since dropped,
// and the person it once named is not reachable through it, so the room takes
// it back. Membership is re-addable, which is why this sweeps rather than asks.
func (service *Service) removeSeatsNobodyAccountsFor(ctx context.Context, relay *sql.DB, channelIDs []string, seed string) {
	report, errorValue := service.sweepSeatsNobodyAccountsFor(ctx, relay, channelIDs, seed, true)
	if errorValue != nil {
		log.Printf("buzz membership: no seat is swept, %v", errorValue)
		return
	}
	if report.Removed > 0 {
		log.Printf("buzz membership: took back %d seat(s) nobody accounts for: %v", report.Removed, report.Seats)
	}
}

type buzzSeatSweepReport struct {
	Rooms   int      `json:"rooms"`
	Seats   []string `json:"seats"`
	Removed int      `json:"removed"`
}

func (service *Service) sweepSeatsNobodyAccountsFor(
	ctx context.Context,
	relay *sql.DB,
	channelIDs []string,
	seed string,
	apply bool,
) (buzzSeatSweepReport, error) {
	accounted, errorValue := service.accountedBuzzPubkeys(ctx, seed)
	if errorValue != nil {
		return buzzSeatSweepReport{}, errorValue
	}
	report := buzzSeatSweepReport{Rooms: len(channelIDs), Seats: []string{}}
	for _, channelID := range channelIDs {
		heldRoles, errorValue := buzzChannelMemberRoles(ctx, relay, channelID)
		if errorValue != nil {
			return report, errorValue
		}
		strangers := seatsNobodyAccountsFor(heldRoles, accounted)
		if len(strangers) == 0 {
			continue
		}
		report.Seats = append(report.Seats, strangers...)
		if !apply {
			continue
		}
		removed, errorValue := removeBuzzChannelMembers(ctx, relay, channelID, strangers)
		if errorValue != nil {
			return report, errorValue
		}
		report.Removed += removed
		if errorValue := service.tellClientsWhoIsInTheRoom(ctx, relay, channelID); errorValue != nil {
			return report, errorValue
		}
	}
	return report, nil
}

// The daily pass is what keeps a room to the people in it, and this is the same
// pass on demand, for when a day is too long to wait.
func (service *Service) handleBuzzSweepSeats(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebMemberRequest(request) {
		http.Error(responseWriter, "member access required", http.StatusForbidden)
		return
	}
	if !service.canWriteToBuzzRelay() {
		http.Error(responseWriter, "this device cannot write to the buzz relay", http.StatusNotImplemented)
		return
	}
	channelIDs, errorValue := service.buzzStreamChannelsWeOpened(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	relay, errorValue := sql.Open("postgres", strings.TrimSpace(service.Configuration.BuzzDatabaseURL))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	defer relay.Close()
	report, errorValue := service.sweepSeatsNobodyAccountsFor(
		request.Context(), relay, channelIDs, service.buzzKeySeed(), request.URL.Query().Get("apply") == "true")
	if errorValue != nil {
		log.Printf("buzz seat sweep failed: %v", errorValue)
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, report)
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
