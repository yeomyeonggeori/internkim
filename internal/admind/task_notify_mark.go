package admind

import (
	"context"
	"time"
)

const taskNotifyMarkFile = "task-notify.json"

type taskNotifyMarks struct {
	SeededAt string                    `json:"seededAt"`
	Marks    map[string]taskNotifyMark `json:"marks"`
}

type taskNotifyMark struct {
	Status   string `json:"status"`
	MarkedAt string `json:"markedAt"`
}

func (service *Service) taskNotifyBaselineSeeded(ctx context.Context) (bool, error) {
	service.taskNotifyMarkMutex.Lock()
	defer service.taskNotifyMarkMutex.Unlock()

	held, errorValue := service.heldTaskNotifyMarks(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	return held.SeededAt != "", nil
}

func (service *Service) readTaskNotifyMarks(ctx context.Context) (map[string]string, error) {
	service.taskNotifyMarkMutex.Lock()
	defer service.taskNotifyMarkMutex.Unlock()

	held, errorValue := service.heldTaskNotifyMarks(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	statusByTaskRunID := map[string]string{}
	for taskRunID, mark := range held.Marks {
		statusByTaskRunID[taskRunID] = mark.Status
	}
	return statusByTaskRunID, nil
}

func (service *Service) writeTaskNotifyMarks(ctx context.Context, statusByTaskRunID map[string]string, at time.Time) error {
	if len(statusByTaskRunID) == 0 {
		return nil
	}
	service.taskNotifyMarkMutex.Lock()
	defer service.taskNotifyMarkMutex.Unlock()

	held, errorValue := service.heldTaskNotifyMarks(ctx)
	if errorValue != nil {
		return errorValue
	}
	marked := at.UTC().Format(time.RFC3339)
	for taskRunID, status := range statusByTaskRunID {
		held.Marks[taskRunID] = taskNotifyMark{Status: status, MarkedAt: marked}
	}
	return service.writeMachineState(taskNotifyMarkFile, held)
}

func (service *Service) writeTaskNotifyBaseline(ctx context.Context, at time.Time) error {
	service.taskNotifyMarkMutex.Lock()
	defer service.taskNotifyMarkMutex.Unlock()

	held, errorValue := service.heldTaskNotifyMarks(ctx)
	if errorValue != nil {
		return errorValue
	}
	if held.SeededAt != "" {
		return nil
	}
	held.SeededAt = at.UTC().Format(time.RFC3339)
	return service.writeMachineState(taskNotifyMarkFile, held)
}

func (service *Service) forgetStaleTaskNotifyMarks(ctx context.Context, before time.Time) error {
	service.taskNotifyMarkMutex.Lock()
	defer service.taskNotifyMarkMutex.Unlock()

	held, errorValue := service.heldTaskNotifyMarks(ctx)
	if errorValue != nil {
		return errorValue
	}
	stale := before.UTC().Format(time.RFC3339)
	forgotten := false
	for taskRunID, mark := range held.Marks {
		if mark.MarkedAt < stale {
			delete(held.Marks, taskRunID)
			forgotten = true
		}
	}
	if !forgotten {
		return nil
	}
	return service.writeMachineState(taskNotifyMarkFile, held)
}

func (service *Service) heldTaskNotifyMarks(ctx context.Context) (taskNotifyMarks, error) {
	var held taskNotifyMarks
	found, errorValue := service.readMachineState(ctx, taskNotifyMarkFile, &held,
		"the task notifier's marks could not be read, so it adopts the runs there are")
	if errorValue != nil || !found {
		return taskNotifyMarks{Marks: map[string]taskNotifyMark{}}, errorValue
	}
	if held.Marks == nil {
		held.Marks = map[string]taskNotifyMark{}
	}
	return held, nil
}
