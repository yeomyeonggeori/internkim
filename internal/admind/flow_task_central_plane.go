package admind

import (
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
	return true, client.DeleteTask(request.Context(), "email", requesterEmail, taskID)
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
