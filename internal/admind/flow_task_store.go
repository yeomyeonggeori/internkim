package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

func (service *Service) readFlowTasks(ctx context.Context, weekCode string, members []flowMember) ([]flowTask, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id
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
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id
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
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id
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
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id
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
	SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id
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

func (service *Service) writeFlowTask(ctx context.Context, task flowTask) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	participantIDs, errorValue := json.Marshal(task.ParticipantIDs)
	if errorValue != nil {
		return errorValue
	}
	participantNames, errorValue := json.Marshal(task.ParticipantNames)
	if errorValue != nil {
		return errorValue
	}
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = transaction.ExecContext(ctx, `
	INSERT INTO flow_tasks (
		id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	week_code = excluded.week_code,
	owner_id = excluded.owner_id,
	owner_name = excluded.owner_name,
	participant_ids = excluded.participant_ids,
	participant_names = excluded.participant_names,
	business = excluded.business,
	type = excluded.type,
	content = excluded.content,
	goal = excluded.goal,
	size = excluded.size,
	status = excluded.status,
	start_date = excluded.start_date,
	end_date = excluded.end_date,
	flag = excluded.flag,
	request_reason = excluded.request_reason,
	decision_reason = excluded.decision_reason,
	mattermost_post_id = excluded.mattermost_post_id,
	updated_at = excluded.updated_at`,
		task.ID,
		task.WeekCode,
		task.OwnerID,
		task.OwnerName,
		string(participantIDs),
		string(participantNames),
		task.Business,
		task.Type,
		task.Content,
		task.Goal,
		task.Size,
		task.Status,
		task.StartDate,
		task.EndDate,
		task.Flag,
		task.RequestReason,
		task.DecisionReason,
		task.MattermostPostID,
		time.Now().UTC().Format(time.RFC3339),
	)
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := enqueueFlowChannelProjection(ctx, transaction, task.ID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func (service *Service) updateFlowTaskMattermostPostID(ctx context.Context, taskID string, postID string) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, "UPDATE flow_tasks SET mattermost_post_id = ?, updated_at = ? WHERE id = ?", strings.TrimSpace(postID), time.Now().UTC().Format(time.RFC3339), taskID)
	return errorValue
}

func (service *Service) deleteFlowTaskByID(ctx context.Context, taskID string) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue = transaction.ExecContext(ctx, "DELETE FROM flow_channel_outbox WHERE task_id = ?", taskID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if _, errorValue = transaction.ExecContext(ctx, "DELETE FROM flow_tasks WHERE id = ?", taskID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func scanFlowTask(rows *sql.Rows) (flowTask, error) {
	var task flowTask
	var participantIDsDocument string
	var participantNamesDocument string
	errorValue := rows.Scan(
		&task.ID,
		&task.WeekCode,
		&task.OwnerID,
		&task.OwnerName,
		&participantIDsDocument,
		&participantNamesDocument,
		&task.Business,
		&task.Type,
		&task.Content,
		&task.Goal,
		&task.Size,
		&task.Status,
		&task.StartDate,
		&task.EndDate,
		&task.Flag,
		&task.RequestReason,
		&task.DecisionReason,
		&task.MattermostPostID,
	)
	if errorValue != nil {
		return flowTask{}, errorValue
	}
	_ = json.Unmarshal([]byte(participantIDsDocument), &task.ParticipantIDs)
	_ = json.Unmarshal([]byte(participantNamesDocument), &task.ParticipantNames)
	return task, nil
}

func alignFlowTasksWithMembers(tasks []flowTask, members []flowMember) []flowTask {
	memberByID := map[string]flowMember{}
	for _, member := range members {
		memberByID[member.ID] = member
	}
	for index, task := range tasks {
		if owner, found := memberByID[task.OwnerID]; found {
			tasks[index].OwnerName = owner.Name
		}
		names := make([]string, 0, len(task.ParticipantIDs))
		for _, memberID := range task.ParticipantIDs {
			if member, found := memberByID[memberID]; found {
				names = append(names, member.Name)
			}
		}
		if len(names) > 0 {
			tasks[index].ParticipantNames = names
		}
	}
	return tasks
}
