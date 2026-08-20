package admind

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

// What docs/internal/task-sync-direction.md §7 asks to be run and read before
// the agent's reads move to the central plane. A difference between the two
// copies is invisible until the device stops answering, which is why it is
// counted here rather than discovered there.
type flowTaskComparison struct {
	Compared     int
	Identical    int
	DifferingBy  map[string]int
	CentralOnly  int
	DeviceOnly   int
	FirstExample string
}

func (service *Service) compareFlowTasksWithCentralPlane(ctx context.Context) (flowTaskComparison, error) {
	comparison := flowTaskComparison{DifferingBy: map[string]int{}}
	client := service.centralPlane()
	if client == nil {
		return comparison, nil
	}
	reader, errorValue := service.administratorWhoRemoves(ctx)
	if errorValue != nil {
		return comparison, errorValue
	}
	people := service.flowPeopleByID(ctx)
	carried := map[string]bool{}
	seenUpTo := ""
	for {
		changed, errorValue := client.TasksChangedSince(ctx, "mattermost", reader, seenUpTo, flowMirrorBatch)
		if errorValue != nil {
			return comparison, errorValue
		}
		if len(changed) == 0 {
			break
		}
		for _, task := range changed {
			if errorValue := service.compareOneFlowTask(ctx, task, people, carried, &comparison); errorValue != nil {
				return comparison, errorValue
			}
		}
		seenUpTo = changed[len(changed)-1].UpdatedAt
	}
	deviceTaskCount, errorValue := service.countFlowTasks(ctx)
	if errorValue != nil {
		return comparison, errorValue
	}
	comparison.DeviceOnly = deviceTaskCount - len(carried)
	return comparison, nil
}

func (service *Service) compareOneFlowTask(ctx context.Context, changed centralplane.ChangedTask, people map[string]adminUserMutation, carried map[string]bool, comparison *flowTaskComparison) error {
	comparison.Compared++
	deviceTaskID := strings.TrimSpace(changed.DeviceTaskID)
	if deviceTaskID == "" {
		known, errorValue := service.deviceTaskIDForCentralTask(ctx, changed.CentralID)
		if errorValue != nil {
			return errorValue
		}
		deviceTaskID = known
	}
	existing, found, errorValue := service.flowTaskIfPresent(ctx, deviceTaskID)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		comparison.CentralOnly++
		return nil
	}
	carried[deviceTaskID] = true

	differences := flowTaskDifferences(existing, deviceFlowTaskOf(changed, existing, true, people))
	if len(differences) == 0 {
		comparison.Identical++
		return nil
	}
	for _, field := range differences {
		comparison.DifferingBy[field]++
	}
	if comparison.FirstExample == "" {
		comparison.FirstExample = fmt.Sprintf("%s differs in %s", deviceTaskID, strings.Join(differences, ", "))
	}
	return nil
}

func flowTaskDifferences(deviceTask flowTask, centralTask flowTask) []string {
	differing := []string{}
	for field, sides := range map[string][2]string{
		"content":   {deviceTask.Content, centralTask.Content},
		"status":    {deviceTask.Status, centralTask.Status},
		"startDate": {deviceTask.StartDate, centralTask.StartDate},
		"endDate":   {deviceTask.EndDate, centralTask.EndDate},
		"business":  {deviceTask.Business, centralTask.Business},
		"type":      {deviceTask.Type, centralTask.Type},
		"size":      {deviceTask.Size, centralTask.Size},
		"people":    {strings.Join(deviceTask.ParticipantIDs, ","), strings.Join(centralTask.ParticipantIDs, ",")},
	} {
		if sides[0] != sides[1] {
			differing = append(differing, field)
		}
	}
	sort.Strings(differing)
	return differing
}

func (service *Service) reportFlowTaskComparison(ctx context.Context) sshRecoveryCommandResult {
	comparison, errorValue := service.compareFlowTasksWithCentralPlane(ctx)
	if errorValue != nil {
		return sshRecoveryCommandResult{
			Name:   "compare what the two copies say",
			Status: "error",
			Output: errorValue.Error(),
		}
	}
	return sshRecoveryCommandResult{
		Name:   "compare what the two copies say",
		Status: "ok",
		Output: describeFlowTaskComparison(comparison),
	}
}

func describeFlowTaskComparison(comparison flowTaskComparison) string {
	lines := []string{fmt.Sprintf("%d compared, %d identical", comparison.Compared, comparison.Identical)}
	for _, field := range sortedComparisonFields(comparison.DifferingBy) {
		lines = append(lines, fmt.Sprintf("  %s differs on %d", field, comparison.DifferingBy[field]))
	}
	if comparison.CentralOnly > 0 {
		lines = append(lines, fmt.Sprintf("  %d on the board this device does not hold; the mirror is behind", comparison.CentralOnly))
	}
	if comparison.DeviceOnly > 0 {
		lines = append(lines, fmt.Sprintf("  %d here the board does not carry; run flow-central-backfill", comparison.DeviceOnly))
	}
	if comparison.FirstExample != "" {
		lines = append(lines, "  first: "+comparison.FirstExample)
	}
	return strings.Join(lines, "\n")
}

func sortedComparisonFields(differingBy map[string]int) []string {
	fields := make([]string, 0, len(differingBy))
	for field := range differingBy {
		fields = append(fields, field)
	}
	sort.Slice(fields, func(left int, right int) bool {
		if differingBy[fields[left]] != differingBy[fields[right]] {
			return differingBy[fields[left]] > differingBy[fields[right]]
		}
		return fields[left] < fields[right]
	})
	return fields
}
