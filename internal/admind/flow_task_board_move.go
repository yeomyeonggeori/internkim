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

const flowTaskBoardRankStep = 1024

var (
	errFlowTaskBoardMoveInvalidRequest = errors.New("invalid flow task board move request")
	errFlowTaskBoardMoveTaskNotFound   = errors.New("flow task board move task not found")
	errFlowTaskBoardMoveForbidden      = errors.New("flow task board move forbidden")
)

type flowTaskBoardMoveAuthorizer func(flowTask) bool

type flowTaskBoardMove struct {
	movedTask flowTask
	updates   []flowTask
}

func (service *Service) writeFlowTaskBoardMove(ctx context.Context, request flowTaskBoardMoveRequest, authorizer flowTaskBoardMoveAuthorizer) (flowTask, error) {
	request = cleanFlowTaskBoardMoveRequest(request)
	if errorValue := validateFlowTaskBoardMoveRequest(request); errorValue != nil {
		return flowTask{}, errorValue
	}
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return flowTask{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return flowTask{}, errorValue
	}
	tasks, errorValue := readFlowTasksForBoardMoveInTransaction(ctx, transaction)
	if errorValue != nil {
		_ = transaction.Rollback()
		return flowTask{}, errorValue
	}
	movedTask, found := flowTaskByID(tasks, request.TaskID)
	if !found {
		_ = transaction.Rollback()
		return flowTask{}, errFlowTaskBoardMoveTaskNotFound
	}
	if authorizer == nil || !authorizer(movedTask) {
		_ = transaction.Rollback()
		return flowTask{}, errFlowTaskBoardMoveForbidden
	}
	move, errorValue := createFlowTaskBoardMove(tasks, request, flowDateNow())
	if errorValue != nil {
		_ = transaction.Rollback()
		return flowTask{}, errorValue
	}
	for _, task := range move.updates {
		if errorValue := writeFlowTaskBoardMoveUpdateInTransaction(ctx, transaction, move.movedTask.ID, task); errorValue != nil {
			_ = transaction.Rollback()
			return flowTask{}, errorValue
		}
	}
	changedTasks := changedFlowTaskBoardMoveSources(tasks, move.updates)
	sourceKeys, errorValue := flowTasksSummarySourceKeys(changedTasks)
	if errorValue != nil {
		_ = transaction.Rollback()
		return flowTask{}, errorValue
	}
	if errorValue := incrementFlowSummarySourceRevisions(ctx, transaction, sourceKeys); errorValue != nil {
		_ = transaction.Rollback()
		return flowTask{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return flowTask{}, errorValue
	}
	return move.movedTask, nil
}

func changedFlowTaskBoardMoveSources(previousTasks []flowTask, updates []flowTask) []flowTask {
	sourceTasks := []flowTask{}
	for _, updatedTask := range updates {
		previousTask, found := flowTaskByID(previousTasks, updatedTask.ID)
		if !found || (previousTask.Status == updatedTask.Status && previousTask.StatusRank == updatedTask.StatusRank) {
			continue
		}
		sourceTasks = append(sourceTasks, previousTask, updatedTask)
	}
	return sourceTasks
}

func writeFlowTaskBoardMoveUpdateInTransaction(ctx context.Context, transaction *sql.Tx, movedTaskID string, task flowTask) error {
	if task.ID == movedTaskID {
		return writeFlowTaskInTransaction(ctx, transaction, task)
	}
	return writeFlowTaskStatusRankInTransaction(ctx, transaction, task)
}

func cleanFlowTaskBoardMoveRequest(request flowTaskBoardMoveRequest) flowTaskBoardMoveRequest {
	request.TaskID = strings.TrimSpace(request.TaskID)
	request.TargetStatus = cleanFlowStatus(request.TargetStatus)
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

func validateFlowTaskBoardMoveRequest(request flowTaskBoardMoveRequest) error {
	if request.TaskID == "" {
		return fmt.Errorf("%w: task id is required", errFlowTaskBoardMoveInvalidRequest)
	}
	if !isFlowTaskBoardMoveStatus(request.TargetStatus) {
		return fmt.Errorf("%w: target status is not movable on the board", errFlowTaskBoardMoveInvalidRequest)
	}
	if request.BeforeTaskID != nil && *request.BeforeTaskID == request.TaskID {
		return fmt.Errorf("%w: before task cannot be the moved task", errFlowTaskBoardMoveInvalidRequest)
	}
	return nil
}

func readFlowTasksForBoardMoveInTransaction(ctx context.Context, transaction *sql.Tx) ([]flowTask, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, status_rank, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id, created_at
FROM flow_tasks`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	tasks := []flowTask{}
	for rows.Next() {
		task, errorValue := scanFlowTask(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func createFlowTaskBoardMove(tasks []flowTask, request flowTaskBoardMoveRequest, now time.Time) (flowTaskBoardMove, error) {
	movedTask, found := flowTaskByID(tasks, request.TaskID)
	if !found {
		return flowTaskBoardMove{}, errFlowTaskBoardMoveTaskNotFound
	}
	targetTasks := flowTasksForBoardStatus(tasks, request.TargetStatus)
	targetTasks = flowTasksExcludingTask(targetTasks, request.TaskID)
	insertIndex, errorValue := flowTaskBoardInsertIndex(targetTasks, request.BeforeTaskID)
	if errorValue != nil {
		return flowTaskBoardMove{}, errorValue
	}
	movedTargetTask := flowTaskWithBoardMoveStatus(movedTask, request.TargetStatus, now)
	reorderedTasks := make([]flowTask, 0, len(targetTasks)+1)
	reorderedTasks = append(reorderedTasks, targetTasks[:insertIndex]...)
	reorderedTasks = append(reorderedTasks, movedTargetTask)
	reorderedTasks = append(reorderedTasks, targetTasks[insertIndex:]...)
	if movedTask.Status == request.TargetStatus && sameFlowTaskOrder(flowTasksForBoardStatus(tasks, request.TargetStatus), reorderedTasks) {
		return flowTaskBoardMove{movedTask: movedTask}, nil
	}
	if statusRank, ok := flowTaskBoardStatusRankBetween(targetTasks, insertIndex); ok {
		movedTargetTask.StatusRank = statusRank
		if !hasFlowTaskBoardMoveChange(movedTargetTask, tasks) {
			return flowTaskBoardMove{movedTask: movedTask}, nil
		}
		return flowTaskBoardMove{
			movedTask: movedTargetTask,
			updates:   []flowTask{movedTargetTask},
		}, nil
	}
	rankedTasks := rankedFlowTaskBoardTasks(reorderedTasks)
	updates := changedFlowTaskBoardTasks(rankedTasks, tasks)
	return flowTaskBoardMove{
		movedTask: flowTaskByIDOrFallback(rankedTasks, movedTargetTask),
		updates:   updates,
	}, nil
}

func flowTaskWithBoardMoveStatus(task flowTask, targetStatus string, now time.Time) flowTask {
	task.Status = targetStatus
	dates := normalizeFlowStatusDates(task.StartDate, task.EndDate, task.WeekCode, targetStatus, now)
	task.StartDate = dates.StartDate
	task.EndDate = dates.EndDate
	task.WeekCode = dates.WeekCode
	return task
}

func isFlowTaskBoardMoveStatus(status string) bool {
	switch cleanFlowStatus(status) {
	case flowStatusRequested, flowStatusPlanned, flowStatusInProgress, flowStatusCompleted, flowStatusPaused:
		return true
	default:
		return false
	}
}

func flowTasksForBoardStatus(tasks []flowTask, status string) []flowTask {
	matchingTasks := []flowTask{}
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

func flowTasksExcludingTask(tasks []flowTask, taskID string) []flowTask {
	filteredTasks := []flowTask{}
	for _, task := range tasks {
		if task.ID != taskID {
			filteredTasks = append(filteredTasks, task)
		}
	}
	return filteredTasks
}

func flowTaskBoardInsertIndex(tasks []flowTask, beforeTaskID *string) (int, error) {
	if beforeTaskID == nil {
		return len(tasks), nil
	}
	for index, task := range tasks {
		if task.ID == *beforeTaskID {
			return index, nil
		}
	}
	return 0, fmt.Errorf("%w: before task is not in target status", errFlowTaskBoardMoveInvalidRequest)
}

func sameFlowTaskOrder(left []flowTask, right []flowTask) bool {
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

func flowTaskBoardStatusRankBetween(tasks []flowTask, insertIndex int) (int, bool) {
	if len(tasks) == 0 {
		return flowTaskBoardRankStep, true
	}
	if insertIndex == 0 {
		if tasks[0].StatusRank <= 1 {
			return 0, false
		}
		return tasks[0].StatusRank / 2, true
	}
	if insertIndex == len(tasks) {
		return tasks[len(tasks)-1].StatusRank + flowTaskBoardRankStep, true
	}
	distance := tasks[insertIndex].StatusRank - tasks[insertIndex-1].StatusRank
	if distance <= 1 {
		return 0, false
	}
	return tasks[insertIndex-1].StatusRank + distance/2, true
}

func rankedFlowTaskBoardTasks(tasks []flowTask) []flowTask {
	rankedTasks := make([]flowTask, 0, len(tasks))
	for index, task := range tasks {
		task.StatusRank = (index + 1) * flowTaskBoardRankStep
		rankedTasks = append(rankedTasks, task)
	}
	return rankedTasks
}

func changedFlowTaskBoardTasks(nextTasks []flowTask, previousTasks []flowTask) []flowTask {
	updates := []flowTask{}
	for _, task := range nextTasks {
		if hasFlowTaskBoardMoveChange(task, previousTasks) {
			updates = append(updates, task)
		}
	}
	return updates
}

func hasFlowTaskBoardMoveChange(task flowTask, previousTasks []flowTask) bool {
	previousTask, found := flowTaskByID(previousTasks, task.ID)
	if !found {
		return false
	}
	return previousTask.Status != task.Status || previousTask.StatusRank != task.StatusRank
}

func flowTaskByIDOrFallback(tasks []flowTask, fallback flowTask) flowTask {
	task, found := flowTaskByID(tasks, fallback.ID)
	if !found {
		return fallback
	}
	return task
}
