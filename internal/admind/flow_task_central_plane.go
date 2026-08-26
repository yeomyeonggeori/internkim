package admind

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

// The company holds the board, so a task is written there as the person who
// asked and the answer comes back carrying the identifier the company gave it.
// This is the path an event already takes; a task and an event are one row.
func (service *Service) saveCentralFlowTask(request *http.Request, task flowTask, people map[string]adminUserMutation) (flowTask, bool, error) {
	requesterEmail := strings.ToLower(strings.TrimSpace(request.Header.Get("CF-Access-Authenticated-User-Email")))
	if requesterEmail == "" {
		return flowTask{}, false, nil
	}
	client := service.centralPlane()
	if client == nil {
		return flowTask{}, false, nil
	}
	savedID, errorValue := client.SaveTask(request.Context(), centralplane.Task{
		CentralID:        centralTaskIdentityOf(task),
		ActorPlatform:    "email",
		ActorExternalID:  requesterEmail,
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
	if errors.Is(errorValue, centralplane.ErrIdentityNotHeld) {
		// The company does not hold this person's board, so the device keeps it.
		// The read path already answers from the device for the same person, and
		// a board somebody can read but not write to is worse than a local one.
		log.Printf("the board stays on this device for %s: %v", requesterEmail, errorValue)
		return flowTask{}, false, nil
	}
	if errorValue != nil {
		return flowTask{}, true, errorValue
	}
	saved := task
	saved.ID = savedID
	return saved, true, nil
}

func (service *Service) removeCentralFlowTask(request *http.Request, taskID string) (bool, error) {
	requesterEmail := strings.ToLower(strings.TrimSpace(request.Header.Get("CF-Access-Authenticated-User-Email")))
	if requesterEmail == "" {
		return false, nil
	}
	client := service.centralPlane()
	if client == nil {
		return false, nil
	}
	errorValue := client.DeleteTask(request.Context(), "email", requesterEmail, taskID)
	if errors.Is(errorValue, centralplane.ErrIdentityNotHeld) {
		log.Printf("the board stays on this device for %s: %v", requesterEmail, errorValue)
		return false, nil
	}
	return true, errorValue
}

// A task the company already holds carries the identifier the company gave it;
// one it has never seen carries none. Sending the device's own asks the company
// to change a row nobody has.
func centralTaskIdentityOf(task flowTask) string {
	identity := strings.TrimSpace(task.ID)
	if len(identity) == 36 && strings.Count(identity, "-") == 4 {
		return identity
	}
	return ""
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

// The people a task names are device identifiers; the central plane knows
// addresses. Anyone the org chart does not carry is left out, and task_save
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
