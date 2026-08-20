package admind

import (
	"sort"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

var deviceStatusOfCentralStatus = map[string]string{
	"todo":        "예정",
	"in_progress": "진행",
	"done":        "완료",
	"paused":      "일시정지",
	"cancelled":   "중단",
}

// The five the enum has become five of the device's seven. 요청 and 기각 are the
// two the central plane cannot tell apart, so a task already sitting in one of
// them keeps it rather than being moved by a change that said nothing about it.
func deviceFlowStatus(centralStatus string, existingDeviceStatus string) string {
	central := strings.TrimSpace(centralStatus)
	existing := strings.TrimSpace(existingDeviceStatus)
	if centralFlowStatus(existing) == central && existing != "" {
		return existing
	}
	if device, known := deviceStatusOfCentralStatus[central]; known {
		return device
	}
	return "예정"
}

func deviceFlowTaskOf(changed centralplane.ChangedTask, existing flowTask, found bool, people map[string]adminUserMutation) flowTask {
	task := existing
	if !found {
		task = flowTask{ID: changed.DeviceTaskID}
	}
	task.Content = changed.Title
	task.Status = deviceFlowStatus(changed.Status, existing.Status)
	task.Business = changed.Business
	task.Type = changed.Type
	task.Size = changed.Size
	dates := normalizeFlowStatusDates(flowDayOf(changed.StartsAt), flowDayOf(changed.EndsAt), existing.WeekCode, task.Status, flowDateNow())
	task.StartDate = dates.StartDate
	task.EndDate = dates.EndDate
	task.WeekCode = dates.WeekCode
	task.Goal, task.RequestReason = flowGoalAndReasonOf(changed.Note)
	task.ParticipantIDs, task.ParticipantNames = flowPeopleOf(changed.ParticipantMails, people)
	if len(task.ParticipantIDs) > 0 && strings.TrimSpace(task.OwnerID) == "" {
		task.OwnerID = task.ParticipantIDs[0]
		task.OwnerName = task.ParticipantNames[0]
	}
	return flowTaskWithCreatedAt(flowTaskWithIdentifier(task))
}

func flowDayOf(moment string) string {
	trimmed := strings.TrimSpace(moment)
	if len(trimmed) < 10 {
		return ""
	}
	return trimmed[:10]
}

// centralFlowNote joins them with the goal first, so they come apart the same way.
func flowGoalAndReasonOf(note string) (string, string) {
	lines := strings.SplitN(strings.TrimSpace(note), "\n", 2)
	goal := ""
	reason := ""
	if strings.HasPrefix(lines[0], "목표: ") {
		goal = strings.TrimPrefix(lines[0], "목표: ")
		if len(lines) > 1 {
			reason = lines[1]
		}
		return goal, reason
	}
	return "", strings.TrimSpace(note)
}

func flowPeopleOf(addresses []string, people map[string]adminUserMutation) ([]string, []string) {
	identifiers := []string{}
	names := []string{}
	for _, address := range sortedAddresses(addresses) {
		identifier := stableFlowID(strings.ToLower(strings.TrimSpace(address)))
		person, known := people[identifier]
		if !known {
			continue
		}
		identifiers = append(identifiers, identifier)
		names = append(names, flowPersonName(person))
	}
	return identifiers, names
}

func sortedAddresses(addresses []string) []string {
	ordered := append([]string{}, addresses...)
	sort.Strings(ordered)
	return ordered
}

func flowPersonName(person adminUserMutation) string {
	if name := strings.TrimSpace(person.Name); name != "" {
		return name
	}
	return strings.TrimSpace(person.Email)
}
