package admind

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/lib/pq"

	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
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
	if report.Removed > 0 || report.LeftTheCommunity > 0 {
		log.Printf("buzz membership: took back %d seat(s) and %d community place(s) nobody accounts for: %v %v",
			report.Removed, report.LeftTheCommunity, report.Seats, report.FormerColleagues)
	}
}

type buzzSeatSweepReport struct {
	Rooms            int      `json:"rooms"`
	Seats            []string `json:"seats"`
	FormerColleagues []string `json:"formerColleagues"`
	Removed          int      `json:"removed"`
	LeftTheCommunity int      `json:"leftTheCommunity"`
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
	formerColleagues, errorValue := service.keysOfMembersWhoLeft(ctx, seed, accounted)
	if errorValue != nil {
		return buzzSeatSweepReport{}, errorValue
	}
	report := buzzSeatSweepReport{Rooms: len(channelIDs), Seats: []string{}, FormerColleagues: formerColleagues}
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
	if apply {
		left, errorValue := showOutOfTheCommunity(ctx, relay, append(report.Seats, report.FormerColleagues...))
		if errorValue != nil {
			return report, errorValue
		}
		report.LeftTheCommunity = left
	}
	return report, nil
}

func (service *Service) keysOfMembersWhoLeft(ctx context.Context, seed string, accounted map[string]bool) ([]string, error) {
	members, errorValue := service.companyMembers(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	keys := []string{}
	for _, member := range members {
		if !member.HasLeftTheCompany() {
			continue
		}
		held, errorValue := service.everyKeyHeldBy(ctx, seed, normalizedRosterEmail(member.Email))
		if errorValue != nil {
			return nil, errorValue
		}
		recorded := strings.ToLower(strings.TrimSpace(member.Messenger[blueclawruntime.BlueclawMessengerPlatform]))
		for _, pubkey := range append(held, recorded) {
			if pubkey != "" && !accounted[pubkey] {
				keys = append(keys, pubkey)
			}
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func showOutOfTheCommunity(ctx context.Context, relay *sql.DB, pubkeys []string) (int, error) {
	if len(pubkeys) == 0 {
		return 0, nil
	}
	result, errorValue := relay.ExecContext(ctx,
		"DELETE FROM relay_members WHERE pubkey = ANY($1) AND role <> 'owner'", pq.Array(pubkeys))
	if errorValue != nil {
		return 0, errorValue
	}
	left, errorValue := result.RowsAffected()
	return int(left), errorValue
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
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
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
