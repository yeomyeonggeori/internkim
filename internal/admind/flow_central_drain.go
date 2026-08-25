package admind

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const (
	flowCentralDrainInterval = time.Minute
	flowCentralDrainBatch    = 25
)

// The queue is drained on an interval rather than at the moment of writing,
// because the write must land whether or not the link is up. See
// docs/internal/task-sync-direction.md §3.
func (service *Service) keepFlowTasksDrained(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(flowCentralDrainInterval):
		}
		service.drainFlowTasksToCentralPlane(ctx)
	}
}

func (service *Service) drainFlowTasksToCentralPlane(ctx context.Context) {
	client := service.centralPlane()
	if client == nil {
		return
	}
	entries, errorValue := service.readFlowCentralOutbox(ctx, flowCentralDrainBatch)
	if errorValue != nil {
		log.Printf("flow queue unreadable: %v", errorValue)
		return
	}
	for _, entry := range entries {
		if errorValue := service.drainOneFlowTask(ctx, client, entry); errorValue != nil {
			service.recordFlowCentralDrainFailure(ctx, entry, errorValue)
			continue
		}
		if errorValue := service.deleteFlowCentralOutbox(ctx, entry.TaskID); errorValue != nil {
			log.Printf("flow queue entry for %s stayed after it was carried: %v", entry.TaskID, errorValue)
		}
	}
}

func (service *Service) recordFlowCentralDrainFailure(ctx context.Context, entry flowCentralOutboxEntry, failure error) {
	log.Print(flowCentralDrainFailureLine(entry, failure))
	if markError := service.markFlowCentralOutboxAttempt(ctx, entry.TaskID, failure); markError != nil {
		log.Printf("flow queue attempt unrecorded for %s: %v", entry.TaskID, markError)
	}
}

func flowCentralDrainFailureLine(entry flowCentralOutboxEntry, failure error) string {
	attempt := entry.AttemptCount + 1
	if attempt < flowCentralOutboxAttemptLimit {
		return fmt.Sprintf("flow task %s did not reach the central plane on attempt %d: %v", entry.TaskID, attempt, failure)
	}
	return fmt.Sprintf("flow task %s is held back after %d attempts and will not be tried again: %v; `internkim recover ssh --action flow-central-held` lists what the queue is holding", entry.TaskID, attempt, failure)
}

func (service *Service) drainOneFlowTask(ctx context.Context, client *centralplane.Client, entry flowCentralOutboxEntry) error {
	if entry.Intent == flowCentralDeleteIntent {
		return service.carryFlowTaskRemoval(ctx, client, entry.TaskID)
	}
	return service.carryFlowTaskWrite(ctx, client, entry.TaskID)
}

func (service *Service) carryFlowTaskWrite(ctx context.Context, client *centralplane.Client, taskID string) error {
	task, found, errorValue := service.readFlowTaskByID(ctx, taskID)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return nil
	}
	people := service.flowPeopleByID(ctx)
	owner, known := people[task.OwnerID]
	if !known || strings.TrimSpace(owner.MattermostUserID) == "" {
		return fmt.Errorf("task %s belongs to %s, who has no messenger account here", taskID, task.OwnerID)
	}

	centralID, errorValue := service.centralIdentityOfFlowTask(ctx, client, taskID, owner.MattermostUserID)
	if errorValue != nil {
		return errorValue
	}
	saved, errorValue := client.SaveTask(ctx, centralplane.Task{
		CentralID:        centralID,
		ActorPlatform:    "mattermost",
		ActorExternalID:  owner.MattermostUserID,
		Title:            task.Content,
		Status:           centralFlowStatus(task.Status),
		Note:             centralFlowNote(task),
		Business:         task.Business,
		Type:             task.Type,
		Size:             task.Size,
		StartsAt:         task.StartDate,
		EndsAt:           task.EndDate,
		WritesDates:      strings.TrimSpace(task.StartDate) != "" || strings.TrimSpace(task.EndDate) != "",
		ParticipantMails: participantAddresses(task, people),
	})
	if errorValue != nil {
		return errorValue
	}
	if centralID == "" {
		if errorValue := client.MarkCarriedFrom(ctx, "mattermost", owner.MattermostUserID, saved, taskID); errorValue != nil {
			return errorValue
		}
	}
	return service.rememberFlowCentralIdentityForTask(ctx, taskID, saved)
}

// The device's tasks were carried over once by hand, so the central plane knows
// which of its rows came from which device task while the device does not. Asking
// before writing is what keeps a first edit from making a second copy, and it
// restores the link after the device loses what it remembered.
func (service *Service) centralIdentityOfFlowTask(ctx context.Context, client *centralplane.Client, taskID string, ownerAccount string) (string, error) {
	remembered, errorValue := service.readFlowCentralIdentityByTaskID(ctx, taskID)
	if errorValue != nil || remembered != "" {
		return remembered, errorValue
	}
	carried, errorValue := client.TaskCarrying(ctx, "mattermost", ownerAccount, taskID)
	if errorValue != nil {
		return "", errorValue
	}
	if carried == "" {
		return "", nil
	}
	return carried, service.rememberFlowCentralIdentityForTask(ctx, taskID, carried)
}

func (service *Service) carryFlowTaskRemoval(ctx context.Context, client *centralplane.Client, taskID string) error {
	centralID, errorValue := service.readFlowCentralIdentityByTaskID(ctx, taskID)
	if errorValue != nil {
		return errorValue
	}
	remover, errorValue := service.administratorWhoRemoves(ctx)
	if errorValue != nil {
		return errorValue
	}
	if centralID == "" {
		// The device forgot which row it wrote, and the company remembers which
		// device task each of its rows came from, so it is asked rather than the
		// removal being reported as carried.
		centralID, errorValue = client.TaskCarrying(ctx, "mattermost", remover, taskID)
		if errorValue != nil {
			return errorValue
		}
	}
	if centralID == "" {
		return fmt.Errorf("the company holds no task carried from %q, so its removal cannot be carried", taskID)
	}
	if errorValue := client.DeleteTask(ctx, "mattermost", remover, centralID); errorValue != nil {
		return errorValue
	}
	return service.forgetFlowCentralIdentityForTask(ctx, taskID)
}

// The people a task names are device identifiers; the central plane knows
// addresses. Anyone the org chart does not carry is left out, and save_flow_task
// decides whether the rest may be there.
func participantAddresses(task flowTask, people map[string]adminUserMutation) []string {
	addresses := []string{}
	for _, participantID := range task.ParticipantIDs {
		person, known := people[participantID]
		if !known || strings.TrimSpace(person.Email) == "" {
			continue
		}
		addresses = append(addresses, person.Email)
	}
	return addresses
}

// A session is issued for a messenger account, which the account directory
// carries and the organization chart drops. Only the request's context is read.
func (service *Service) flowPeopleByID(ctx context.Context) map[string]adminUserMutation {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	if errorValue != nil {
		return map[string]adminUserMutation{}
	}
	people := map[string]adminUserMutation{}
	for _, record := range service.accountDirectoryUserRecords(request) {
		people[flowMemberIdentifier(record)] = record
	}
	return people
}

// A removal is recorded as whoever made it, so the same administrator makes it
// every time rather than whichever one a map happened to yield first.
func (service *Service) administratorWhoRemoves(ctx context.Context) (string, error) {
	account := ""
	address := ""
	for _, person := range service.flowPeopleByID(ctx) {
		if !strings.EqualFold(person.Role, "admin") || strings.TrimSpace(person.MattermostUserID) == "" {
			continue
		}
		if address == "" || strings.ToLower(person.Email) < address {
			address = strings.ToLower(person.Email)
			account = strings.TrimSpace(person.MattermostUserID)
		}
	}
	if account == "" {
		return "", fmt.Errorf("no administrator here has a messenger account, so nothing can be removed centrally")
	}
	return account, nil
}

func (service *Service) readFlowCentralIdentityByTaskID(ctx context.Context, taskID string) (string, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	defer database.Close()
	return readFlowCentralIdentity(ctx, database, taskID)
}

func (service *Service) rememberFlowCentralIdentityForTask(ctx context.Context, taskID string, centralTaskID string) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	return rememberFlowCentralIdentity(ctx, database, taskID, centralTaskID)
}

func (service *Service) forgetFlowCentralIdentityForTask(ctx context.Context, taskID string) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	return forgetFlowCentralIdentity(ctx, database, taskID)
}
