package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

// A company writes every row as the person who asked, so a caller this device
// cannot name has nowhere to write. Falling back to the device's own store
// would answer that caller with a row the company never receives and nothing
// afterwards reconciles.
var errTaskWriterUnnamed = errors.New("this write names nobody the company knows, and the company keeps the board")

// A task is added through the record's task_add, so the rules every other
// path meets (labels, requests, duplicates) are met here too.
func (service *Service) addTaskThroughTheRecord(request *http.Request, task Task, note string, people map[string]adminUserMutation) (centralplane.RecordToolAnswer, bool, error) {
	client := service.centralPlane()
	if client == nil {
		return centralplane.RecordToolAnswer{}, false, nil
	}
	requesterEmail := service.taskActorEmail(request)
	if requesterEmail == "" {
		return centralplane.RecordToolAnswer{}, true, errTaskWriterUnnamed
	}
	input, errorValue := json.Marshal(taskAddInputOf(task, note, participantAddresses(task, people)))
	if errorValue != nil {
		return centralplane.RecordToolAnswer{}, true, errorValue
	}
	answer, errorValue := client.InvokeRecordTool(request.Context(), requesterEmail, "task_add", "invoke", input)
	return answer, true, errorValue
}

func taskAddInputOf(task Task, note string, participantAddresses []string) map[string]any {
	input := map[string]any{"title": task.Content, "participantPersonHints": participantAddresses}
	given := map[string]string{
		"note":     note,
		"status":   statusTaskAddTakes(task.Status),
		"business": task.Business,
		"type":     task.Type,
		"size":     task.Size,
		"startsAt": task.StartDate,
		"endsAt":   task.EndDate,
	}
	for name, value := range given {
		if strings.TrimSpace(value) != "" {
			input[name] = value
		}
	}
	return input
}

func statusTaskAddTakes(status string) string {
	if status == taskStatusRequested || status == taskStatusPlanned {
		return ""
	}
	return status
}

// A session is issued for a messenger account, which the account directory
// carries and the organization chart drops. Only the request's context is read.
func (service *Service) taskPeopleByID(ctx context.Context) map[string]adminUserMutation {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	if errorValue != nil {
		return map[string]adminUserMutation{}
	}
	people := map[string]adminUserMutation{}
	for _, record := range service.accountDirectoryUserRecords(request) {
		people[taskMemberIdentifier(record)] = record
	}
	return people
}

// The people a task names are device identifiers; the central plane knows
// addresses. Anyone the org chart does not carry is left out, and task_save
// decides whether the rest may be there.
func participantAddresses(task Task, people map[string]adminUserMutation) []string {
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
