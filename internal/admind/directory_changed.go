package admind

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"
)

func (service *Service) handleDirectoryChanged(responseWriter http.ResponseWriter, request *http.Request) {
	log.Printf("the company says its directory changed, so this device reads it again")
	service.triggerUsersSync(request.Context())
	recording := service.recordBuzzCredentials(request.Context())
	log.Printf("buzz credentials after the directory changed: %s", recording)
	go service.showOutWhoeverLeftTheCompany(context.Background())
	responseWriter.WriteHeader(http.StatusAccepted)
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
