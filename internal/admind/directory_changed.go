package admind

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

var (
	directoryApplyInterval  = 2 * time.Minute
	firstDirectoryRetryWait = 250 * time.Millisecond
)

func (service *Service) handleDirectoryChanged(responseWriter http.ResponseWriter, request *http.Request) {
	log.Printf("the company says its directory changed, so this device applies it again")
	if errorValue := service.applyDirectory(request.Context()); errorValue != nil {
		log.Printf("the directory is not fully applied: %v", errorValue)
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	responseWriter.WriteHeader(http.StatusAccepted)
}

func (service *Service) startDirectoryApplying(ctx context.Context) {
	go service.keepTheDirectoryApplied(ctx)
}

func (service *Service) keepTheDirectoryApplied(ctx context.Context) {
	retryWait := firstDirectoryRetryWait
	for {
		wait := directoryApplyInterval
		if errorValue := service.applyDirectoryWithinABudget(ctx); errorValue != nil {
			log.Printf("the directory is not fully applied, trying again in %s: %v", retryWait, errorValue)
			wait, retryWait = retryWait, min(retryWait*2, directoryApplyInterval)
		} else {
			retryWait = firstDirectoryRetryWait
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func (service *Service) applyDirectoryWithinABudget(ctx context.Context) error {
	passContext, cancel := context.WithTimeout(ctx, buzzDatabaseSweepBudget)
	defer cancel()
	return service.applyDirectory(passContext)
}

func (service *Service) applyDirectory(ctx context.Context) error {
	service.directoryApplying.Lock()
	defer service.directoryApplying.Unlock()
	client := service.centralPlane()
	if client == nil {
		return nil
	}
	members, errorValue := client.Members(ctx)
	if errorValue != nil {
		service.recordRosterReconcile(errorValue)
		return fmt.Errorf("the company did not say who works here: %w", errorValue)
	}
	return errors.Join(
		named("roster", service.applyRoster(ctx, members)),
		service.applyDirectoryToTheMessenger(ctx, client, members),
	)
}

func (service *Service) applyDirectoryToTheMessenger(ctx context.Context, client *centralplane.Client, members []centralplane.Member) error {
	if !service.canWriteToBuzzRelay() {
		return nil
	}
	return errors.Join(
		named("messenger keys", service.recordMessengerKeys(ctx, client, members)),
		named("messenger seats", service.ensureMemberChannelMembership(ctx)),
		named("circle rooms", service.keepCircleChannelsToTheirCircles(ctx)),
		named("messenger names", service.nameEveryoneTheDirectoryNames(ctx)),
		named("keys nobody names", service.showOutWhoeverNobodyNames(ctx)),
		named("messenger account links", service.linkDeterministicBuzzPeople(ctx)),
	)
}

func named(step string, errorValue error) error {
	if errorValue == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", step, errorValue)
}
