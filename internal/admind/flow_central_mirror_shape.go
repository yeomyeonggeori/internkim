package admind

import (
	"sort"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

// One word each way, so the reverse is the inverse and there is no second map to
// keep in step by hand.
var deviceStatusOfCentralStatus = invertStatusWords()

func invertStatusWords() map[string]string {
	inverted := map[string]string{}
	for deviceStatus, centralStatus := range centralStatusOfDeviceStatus {
		inverted[centralStatus] = deviceStatus
	}
	return inverted
}

func deviceFlowStatus(centralStatus string) string {
	if deviceStatus, known := deviceStatusOfCentralStatus[strings.TrimSpace(centralStatus)]; known {
		return deviceStatus
	}
	return "예정"
}

func deviceFlowTaskOf(changed centralplane.ChangedTask, existing flowTask, found bool, people map[string]adminUserMutation) flowTask {
	task := existing
	if !found {
		task = flowTask{ID: changed.DeviceTaskID}
	}
	task.Content = changed.Title
	task.Status = deviceFlowStatus(changed.Status)
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

// The central plane stores a day as its first instant in the company's timezone,
// which is the previous day in UTC. Slicing the string a PostgREST read returns
// therefore names the day before the one meant, for a start but not for an end,
// because 23:59 does not cross midnight going west.
func flowDayOf(moment string) string {
	instant, readable := readFlowTimestamp(moment)
	if !readable {
		return ""
	}
	return instant.In(flowDateTimezone).Format("2006-01-02")
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
