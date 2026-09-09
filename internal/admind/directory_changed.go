package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func (service *Service) handleDirectoryChanged(responseWriter http.ResponseWriter, request *http.Request) {
	log.Printf("the company says its directory changed, so this device reads it again")
	service.triggerUsersSync(request.Context())
	// The roster is what decides who may act at all, and it was the one thing this
	// door did not read again: somebody invited and asking a question in the same
	// minute was told they are not an active member until the two-minute pass came
	// round.
	service.reconcileBlueclawRosterWithTimeout(request.Context())
	recording := service.recordBuzzCredentials(request.Context())
	log.Printf("buzz credentials after the directory changed: %s", recording)
	if service.canWriteToBuzzRelay() {
		service.ensureMemberChannelMembership(request.Context())
	}
	go service.showOutWhoeverLeftTheCompany(context.Background())

	// Answering 202 whatever happened is how somebody stayed unanswerable behind
	// an invitation that reported success. Nobody got a key when nobody could:
	// that is this door failing, and it says so.
	answer := directoryChangedAnswer{
		KeysKept:    recording.Kept,
		KeysSkipped: recording.Skipped,
		KeysRefused: recording.Refusals,
		WasRecorded: recording.Kept > 0 || len(recording.Refusals) == 0,
	}
	status := http.StatusAccepted
	if !answer.WasRecorded {
		status = http.StatusInternalServerError
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(answer)
}

type directoryChangedAnswer struct {
	KeysKept    int      `json:"keysKept"`
	KeysSkipped int      `json:"keysSkipped"`
	KeysRefused []string `json:"keysRefused,omitempty"`
	WasRecorded bool     `json:"wasRecorded"`
}

// Removing somebody from the directory is what makes them a former colleague,
// and a former colleague reads nothing. The daily pass would reach this within
// a day; somebody who has just been removed should not still be in 광장 while
// that day runs out.
func (service *Service) showOutWhoeverLeftTheCompany(ctx context.Context) {
	if !service.canWriteToBuzzRelay() {
		return
	}
	channelIDs, errorValue := service.buzzStreamChannelsWeOpened(ctx)
	if errorValue != nil {
		log.Printf("buzz membership: the rooms could not be read after the directory changed: %v", errorValue)
		return
	}
	relay, errorValue := sql.Open("postgres", strings.TrimSpace(service.Configuration.BuzzDatabaseURL))
	if errorValue != nil {
		log.Printf("buzz membership: %v", errorValue)
		return
	}
	defer relay.Close()
	service.removeSeatsNobodyAccountsFor(ctx, relay, channelIDs, service.buzzKeySeed())
}
