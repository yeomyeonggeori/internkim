package admind

import (
	"context"
	"fmt"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

// The rows the mirror mis-dated carry a device updated_at newer than the central
// one, so §4's later-write-wins rule will never revisit them. Correcting the
// mirror's own damage is why this is allowed to ignore that rule, and why it
// takes only the dates: every other field the device holds may be its own edit.
func (service *Service) repairFlowTaskDatesFromCentralPlane(ctx context.Context) (int, error) {
	client := service.centralPlane()
	if client == nil {
		return 0, nil
	}
	reader, errorValue := service.administratorWhoRemoves(ctx)
	if errorValue != nil {
		return 0, errorValue
	}
	people := service.flowPeopleByID(ctx)
	repaired := 0
	seenUpTo := ""
	for {
		changed, errorValue := client.TasksChangedSince(ctx, "mattermost", reader, seenUpTo, flowMirrorBatch)
		if errorValue != nil {
			return repaired, errorValue
		}
		if len(changed) == 0 {
			return repaired, nil
		}
		for _, task := range changed {
			moved, errorValue := service.repairOneFlowTaskDate(ctx, task, people)
			if errorValue != nil {
				return repaired, errorValue
			}
			if moved {
				repaired++
			}
		}
		seenUpTo = changed[len(changed)-1].UpdatedAt
	}
}

func (service *Service) repairOneFlowTaskDate(ctx context.Context, changed centralplane.ChangedTask, people map[string]adminUserMutation) (bool, error) {
	deviceTaskID, errorValue := service.deviceTaskIDForCentralTask(ctx, changed.CentralID)
	if errorValue != nil {
		return false, errorValue
	}
	if deviceTaskID == "" {
		deviceTaskID = changed.DeviceTaskID
	}
	existing, found, errorValue := service.flowTaskIfPresent(ctx, deviceTaskID)
	if errorValue != nil || !found {
		return false, errorValue
	}

	central := deviceFlowTaskOf(changed, existing, true, people)
	if central.StartDate == existing.StartDate && central.EndDate == existing.EndDate && central.WeekCode == existing.WeekCode {
		return false, nil
	}

	repaired := existing
	repaired.StartDate = central.StartDate
	repaired.EndDate = central.EndDate
	repaired.WeekCode = central.WeekCode
	return true, service.writeMirroredFlowTask(ctx, repaired)
}

func (service *Service) reportFlowTaskDateRepair(ctx context.Context) sshRecoveryCommandResult {
	repaired, errorValue := service.repairFlowTaskDatesFromCentralPlane(ctx)
	if errorValue != nil {
		return sshRecoveryCommandResult{
			Name:   "put the mirrored task dates back",
			Status: "error",
			Output: fmt.Sprintf("%d repaired before failing: %v", repaired, errorValue),
		}
	}
	return sshRecoveryCommandResult{
		Name:   "put the mirrored task dates back",
		Status: "ok",
		Output: fmt.Sprintf("%d repaired", repaired),
	}
}
