package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const taskBoardRankStep = 1024

var (
	errTaskBoardMoveInvalidRequest = errors.New("invalid flow task board move request")
	errTaskBoardMoveTaskNotFound   = errors.New("flow task board move task not found")
	errTaskBoardMoveForbidden      = errors.New("flow task board move forbidden")
)

type taskBoardMoveAuthorizer func(Task) bool

type taskBoardMove struct {
	movedTask Task
	updates   []Task
}

func (service *Service) writeTaskBoardMove(ctx context.Context, request taskBoardMoveRequest, authorizer taskBoardMoveAuthorizer) (Task, error) {
	request = cleanTaskBoardMoveRequest(request)
	if errorValue := validateTaskBoardMoveRequest(request); errorValue != nil {
		return Task{}, errorValue
	}
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return Task{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return Task{}, errorValue
	}
	tasks, errorValue := readAllTasksInTransaction(ctx, transaction)
	if errorValue != nil {
		_ = transaction.Rollback()
		return Task{}, errorValue
	}
	movedTask, found := taskByID(tasks, request.TaskID)
	if !found {
		_ = transaction.Rollback()
		return Task{}, errTaskBoardMoveTaskNotFound
	}
	if authorizer == nil || !authorizer(movedTask) {
		_ = transaction.Rollback()
		return Task{}, errTaskBoardMoveForbidden
	}
	move, errorValue := createTaskBoardMove(tasks, request, taskDateNow())
	if errorValue != nil {
		_ = transaction.Rollback()
		return Task{}, errorValue
	}
	for _, task := range move.updates {
		if errorValue := writeTaskBoardMoveUpdateInTransaction(ctx, transaction, move.movedTask.ID, task); errorValue != nil {
			_ = transaction.Rollback()
			return Task{}, errorValue
		}
	}
	sourceKeys := taskBoardMoveSummarySourceKeys(tasks, move.updates)
	if errorValue := incrementTaskSummarySourceRevisions(ctx, transaction, sourceKeys); errorValue != nil {
		_ = transaction.Rollback()
		return Task{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return Task{}, errorValue
	}
	return move.movedTask, nil
}

func writeTaskBoardMoveUpdateInTransaction(ctx context.Context, transaction *sql.Tx, movedTaskID string, task Task) error {
	if task.ID == movedTaskID {
		return writeTaskInTransaction(ctx, transaction, task)
	}
	return writeTaskStatusRankInTransaction(ctx, transaction, task)
}

func cleanTaskBoardMoveRequest(request taskBoardMoveRequest) taskBoardMoveRequest {
	request.TaskID = strings.TrimSpace(request.TaskID)
	request.TargetStatus = cleanTaskStatus(request.TargetStatus)
	if request.BeforeTaskID == nil {
		return request
	}
	beforeTaskID := strings.TrimSpace(*request.BeforeTaskID)
	if beforeTaskID == "" {
		request.BeforeTaskID = nil
		return request
	}
	request.BeforeTaskID = &beforeTaskID
	return request
}

func validateTaskBoardMoveRequest(request taskBoardMoveRequest) error {
	if request.TaskID == "" {
		return fmt.Errorf("%w: task id is required", errTaskBoardMoveInvalidRequest)
	}
	if !isTaskBoardMoveStatus(request.TargetStatus) {
		return fmt.Errorf("%w: target status is not movable on the board", errTaskBoardMoveInvalidRequest)
	}
	if request.BeforeTaskID != nil && *request.BeforeTaskID == request.TaskID {
		return fmt.Errorf("%w: before task cannot be the moved task", errTaskBoardMoveInvalidRequest)
	}
	return nil
}

func createTaskBoardMove(tasks []Task, request taskBoardMoveRequest, now time.Time) (taskBoardMove, error) {
	movedTask, found := taskByID(tasks, request.TaskID)
	if !found {
		return taskBoardMove{}, errTaskBoardMoveTaskNotFound
	}
	targetTasks := tasksForBoardStatus(tasks, request.TargetStatus)
	targetTasks = tasksExcludingTask(targetTasks, request.TaskID)
	insertIndex, errorValue := taskBoardInsertIndex(targetTasks, request.BeforeTaskID)
	if errorValue != nil {
		return taskBoardMove{}, errorValue
	}
	movedTargetTask := taskWithBoardMoveStatus(movedTask, request.TargetStatus, now)
	reorderedTasks := make([]Task, 0, len(targetTasks)+1)
	reorderedTasks = append(reorderedTasks, targetTasks[:insertIndex]...)
	reorderedTasks = append(reorderedTasks, movedTargetTask)
	reorderedTasks = append(reorderedTasks, targetTasks[insertIndex:]...)
	if movedTask.Status == request.TargetStatus && sameTaskOrder(tasksForBoardStatus(tasks, request.TargetStatus), reorderedTasks) {
		return taskBoardMove{movedTask: movedTask}, nil
	}
	if statusRank, ok := taskBoardStatusRankBetween(targetTasks, insertIndex); ok {
		movedTargetTask.StatusRank = statusRank
		if !hasTaskBoardMoveChange(movedTargetTask, tasks) {
			return taskBoardMove{movedTask: movedTask}, nil
		}
		return taskBoardMove{
			movedTask: movedTargetTask,
			updates:   []Task{movedTargetTask},
		}, nil
	}
	rankedTasks := rankedTaskBoardTasks(reorderedTasks)
	updates := changedTaskBoardTasks(rankedTasks, tasks)
	return taskBoardMove{
		movedTask: taskByIDOrFallback(rankedTasks, movedTargetTask),
		updates:   updates,
	}, nil
}

func taskWithBoardMoveStatus(task Task, targetStatus string, now time.Time) Task {
	task.Status = targetStatus
	dates := normalizeTaskStatusDates(task.StartDate, task.EndDate, task.WeekCode, targetStatus, now)
	task.StartDate = dates.StartDate
	task.EndDate = dates.EndDate
	task.WeekCode = dates.WeekCode
	return task
}

func isTaskBoardMoveStatus(status string) bool {
	switch cleanTaskStatus(status) {
	case taskStatusRequested, taskStatusPlanned, taskStatusInProgress, taskStatusCompleted, taskStatusPaused:
		return true
	default:
		return false
	}
}

func tasksForBoardStatus(tasks []Task, status string) []Task {
	matchingTasks := []Task{}
	for _, task := range tasks {
		if task.Status == status {
			matchingTasks = append(matchingTasks, task)
		}
	}
	sort.Slice(matchingTasks, func(leftIndex int, rightIndex int) bool {
		leftTask := matchingTasks[leftIndex]
		rightTask := matchingTasks[rightIndex]
		if leftTask.StatusRank != rightTask.StatusRank {
			return leftTask.StatusRank < rightTask.StatusRank
		}
		return leftTask.ID < rightTask.ID
	})
	return matchingTasks
}

func tasksExcludingTask(tasks []Task, taskID string) []Task {
	filteredTasks := []Task{}
	for _, task := range tasks {
		if task.ID != taskID {
			filteredTasks = append(filteredTasks, task)
		}
	}
	return filteredTasks
}

func taskBoardInsertIndex(tasks []Task, beforeTaskID *string) (int, error) {
	if beforeTaskID == nil {
		return len(tasks), nil
	}
	for index, task := range tasks {
		if task.ID == *beforeTaskID {
			return index, nil
		}
	}
	return 0, fmt.Errorf("%w: before task is not in target status", errTaskBoardMoveInvalidRequest)
}

func sameTaskOrder(left []Task, right []Task) bool {
	if len(left) != len(right) {
		return false
	}
	for index, task := range left {
		if task.ID != right[index].ID {
			return false
		}
	}
	return true
}

func taskBoardStatusRankBetween(tasks []Task, insertIndex int) (int, bool) {
	if len(tasks) == 0 {
		return taskBoardRankStep, true
	}
	if insertIndex == 0 {
		if tasks[0].StatusRank <= 1 {
			return 0, false
		}
		return tasks[0].StatusRank / 2, true
	}
	if insertIndex == len(tasks) {
		return tasks[len(tasks)-1].StatusRank + taskBoardRankStep, true
	}
	distance := tasks[insertIndex].StatusRank - tasks[insertIndex-1].StatusRank
	if distance <= 1 {
		return 0, false
	}
	return tasks[insertIndex-1].StatusRank + distance/2, true
}

func rankedTaskBoardTasks(tasks []Task) []Task {
	rankedTasks := make([]Task, 0, len(tasks))
	for index, task := range tasks {
		task.StatusRank = (index + 1) * taskBoardRankStep
		rankedTasks = append(rankedTasks, task)
	}
	return rankedTasks
}

func changedTaskBoardTasks(nextTasks []Task, previousTasks []Task) []Task {
	updates := []Task{}
	for _, task := range nextTasks {
		if hasTaskBoardMoveChange(task, previousTasks) {
			updates = append(updates, task)
		}
	}
	return updates
}

func hasTaskBoardMoveChange(task Task, previousTasks []Task) bool {
	previousTask, found := taskByID(previousTasks, task.ID)
	if !found {
		return false
	}
	return previousTask.Status != task.Status || previousTask.StatusRank != task.StatusRank
}

func taskByIDOrFallback(tasks []Task, fallback Task) Task {
	task, found := taskByID(tasks, fallback.ID)
	if !found {
		return fallback
	}
	return task
}
