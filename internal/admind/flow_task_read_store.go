package admind

import (
	"context"
	"database/sql"
)

type flowTaskQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (service *Service) readFlowTasks(ctx context.Context, weekCode string, members []flowMember) ([]flowTask, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, status_rank, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id, created_at
FROM flow_tasks
WHERE week_code = ?
ORDER BY status = '요청' DESC, owner_name, updated_at DESC`, weekCode)
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
	return alignFlowTasksWithMembers(tasks, members), rows.Err()
}

func (service *Service) readAllFlowTasks(ctx context.Context, members []flowMember) ([]flowTask, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, status_rank, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id, created_at
FROM flow_tasks
ORDER BY status = '요청' DESC, week_code DESC, owner_name, updated_at DESC`)
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
	return alignFlowTasksWithMembers(tasks, members), rows.Err()
}

func (service *Service) readFlowTasksBetweenDates(ctx context.Context, startDate string, endDate string, members []flowMember) ([]flowTask, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, status_rank, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id, created_at
FROM flow_tasks
WHERE (start_date >= ? AND start_date <= ?) OR (end_date >= ? AND end_date <= ?)
ORDER BY start_date, owner_name, updated_at DESC`, startDate, endDate, startDate, endDate)
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
	return alignFlowTasksWithMembers(tasks, members), rows.Err()
}

func (service *Service) readFlowTaskByID(ctx context.Context, taskID string) (flowTask, bool, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return flowTask{}, false, errorValue
	}
	defer database.Close()
	return readFlowTaskByIDWithQueryer(ctx, database, taskID)
}

func readFlowTaskByIDInTransaction(ctx context.Context, transaction *sql.Tx, taskID string) (flowTask, bool, error) {
	return readFlowTaskByIDWithQueryer(ctx, transaction, taskID)
}

func readFlowTaskByIDWithQueryer(ctx context.Context, queryer flowTaskQueryer, taskID string) (flowTask, bool, error) {
	rows, errorValue := queryer.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, status_rank, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id, created_at
FROM flow_tasks
WHERE id = ?`, taskID)
	if errorValue != nil {
		return flowTask{}, false, errorValue
	}
	defer rows.Close()
	if !rows.Next() {
		return flowTask{}, false, rows.Err()
	}
	task, errorValue := scanFlowTask(rows)
	if errorValue != nil {
		return flowTask{}, false, errorValue
	}
	return task, true, rows.Err()
}

func (service *Service) readFlowTasksWithMattermostPosts(ctx context.Context) ([]flowTask, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
	SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, status_rank, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id, created_at
	FROM flow_tasks
	WHERE mattermost_post_id != '' OR status IN (?, ?, ?, ?) OR id IN (SELECT task_id FROM flow_channel_outbox)
	ORDER BY updated_at DESC`,
		flowStatusRequested,
		flowStatusCompleted,
		flowStatusRejected,
		flowStatusStopped,
	)
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

func (service *Service) existingFlowMattermostPostID(ctx context.Context, taskID string) string {
	task, found, errorValue := service.readFlowTaskByID(ctx, taskID)
	if errorValue != nil || !found {
		return ""
	}
	return task.MattermostPostID
}
