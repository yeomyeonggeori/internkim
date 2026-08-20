package admind

import (
	"context"
	"log"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const (
	flowMirrorInterval = time.Minute
	flowMirrorBatch    = 50
	flowMirrorMark     = "flow-central-mirror"
)

// Work written on the board has to reach the device, because the agent reads the
// device's copy. See docs/internal/task-sync-direction.md §2.
func (service *Service) keepFlowTasksMirrored(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(flowMirrorInterval):
		}
		service.mirrorFlowTasksFromCentralPlane(ctx)
	}
}

func (service *Service) mirrorFlowTasksFromCentralPlane(ctx context.Context) {
	client := service.centralPlane()
	if client == nil {
		return
	}
	reader, errorValue := service.administratorWhoRemoves(ctx)
	if errorValue != nil {
		log.Printf("the board cannot be read: %v", errorValue)
		return
	}
	seenUpTo, errorValue := service.readFlowMirrorMark(ctx)
	if errorValue != nil {
		log.Printf("flow mirror mark unreadable: %v", errorValue)
		return
	}
	changed, errorValue := client.TasksChangedSince(ctx, "mattermost", reader, seenUpTo, flowMirrorBatch)
	if errorValue != nil {
		log.Printf("the board did not say what changed: %v", errorValue)
		return
	}
	people := service.flowPeopleByID(ctx)
	carried := 0
	for _, task := range changed {
		landed, errorValue := service.mirrorOneFlowTask(ctx, task, people)
		if errorValue != nil {
			log.Printf("task %s did not reach this device: %v", task.CentralID, errorValue)
			return
		}
		if landed {
			carried++
		}
		if errorValue := service.writeFlowMirrorMark(ctx, task.UpdatedAt); errorValue != nil {
			log.Printf("flow mirror mark unwritten: %v", errorValue)
			return
		}
	}
	if len(changed) > 0 {
		log.Printf("the board had %d changes for this device and %d were newer than what it holds, now read up to %s",
			len(changed), carried, changed[len(changed)-1].UpdatedAt)
	}
}

// A task the device already holds keeps its own identifier, so the copy the
// agent reads is the same row it has always been. One the board made arrives
// with none, and is given one.
func (service *Service) mirrorOneFlowTask(ctx context.Context, changed centralplane.ChangedTask, people map[string]adminUserMutation) (bool, error) {
	deviceTaskID := strings.TrimSpace(changed.DeviceTaskID)
	if deviceTaskID == "" {
		known, errorValue := service.deviceTaskIDForCentralTask(ctx, changed.CentralID)
		if errorValue != nil {
			return false, errorValue
		}
		deviceTaskID = known
	}

	existing, found, errorValue := service.flowTaskIfPresent(ctx, deviceTaskID)
	if errorValue != nil {
		return false, errorValue
	}
	if found {
		writtenAt, errorValue := service.flowTaskWrittenAt(ctx, deviceTaskID)
		if errorValue != nil {
			return false, errorValue
		}
		if !centralChangeIsNewer(changed.UpdatedAt, writtenAt) {
			return false, nil
		}
	}

	task := deviceFlowTaskOf(changed, existing, found, people)
	if errorValue := service.writeMirroredFlowTask(ctx, task); errorValue != nil {
		return false, errorValue
	}
	return true, service.rememberFlowCentralIdentityForTask(ctx, task.ID, changed.CentralID)
}

// Both sides may have written while the link was down, and the later write wins.
// See docs/internal/task-sync-direction.md §4.
func centralChangeIsNewer(centralUpdatedAt string, deviceUpdatedAt string) bool {
	central, centralReadable := readFlowTimestamp(centralUpdatedAt)
	device, deviceReadable := readFlowTimestamp(deviceUpdatedAt)
	if !centralReadable {
		return false
	}
	if !deviceReadable {
		return true
	}
	return central.After(device)
}

func readFlowTimestamp(value string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999-07:00"} {
		if moment, errorValue := time.Parse(layout, strings.TrimSpace(value)); errorValue == nil {
			return moment.UTC(), true
		}
	}
	return time.Time{}, false
}

func (service *Service) flowTaskIfPresent(ctx context.Context, taskID string) (flowTask, bool, error) {
	if strings.TrimSpace(taskID) == "" {
		return flowTask{}, false, nil
	}
	return service.readFlowTaskByID(ctx, taskID)
}
