package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type flowTaskQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (service *Service) readFlowTasks(ctx context.Context, weekCode string, members []flowMember) ([]flowTask, error) {
	if tasks, answered := service.companyBoardTasks(ctx, members); answered {
		return flowTasksInWeek(tasks, weekCode), nil
	}
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, calendar_event_id, created_at
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
	if tasks, answered := service.companyBoardTasks(ctx, members); answered {
		return tasks, nil
	}
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, calendar_event_id, created_at
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
	if tasks, answered := service.companyBoardTasks(ctx, members); answered {
		return flowTasksBetweenDays(tasks, startDate, endDate), nil
	}
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, calendar_event_id, created_at
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

// A caller names a task by the identifier it was answered with, which is the
// company's. The device still files its own copy under an identifier of its
// own, so the company's is turned back into it before the row is looked up.
// A task born on the company board has no device row to look up, so a miss in
// the device store is answered from the board the reads already come from.
func (service *Service) readFlowTaskAnswering(ctx context.Context, taskID string, members []flowMember) (flowTask, bool, error) {
	task, found, errorValue := service.readFlowTaskByID(ctx, taskID)
	if found || errorValue != nil {
		return task, found, errorValue
	}
	boardTasks, answered := service.companyBoardTasks(ctx, members)
	if !answered {
		return flowTask{}, false, nil
	}
	for _, boardTask := range boardTasks {
		if boardTask.ID == strings.TrimSpace(taskID) {
			return boardTask, true, nil
		}
	}
	return flowTask{}, false, nil
}

func (service *Service) readFlowTaskByID(ctx context.Context, taskID string) (flowTask, bool, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return flowTask{}, false, errorValue
	}
	defer database.Close()
	if task, found, errorValue := readFlowTaskByIDWithQueryer(ctx, database, taskID); found || errorValue != nil {
		return task, found, errorValue
	}
	deviceTaskID, errorValue := readFlowTaskIDCarrying(ctx, database, taskID)
	if errorValue != nil || deviceTaskID == "" {
		return flowTask{}, false, errorValue
	}
	return readFlowTaskByIDWithQueryer(ctx, database, deviceTaskID)
}

func (service *Service) readFlowTaskByCalendarEventID(ctx context.Context, eventID string) (flowTask, bool, error) {
	trimmedEventID := strings.TrimSpace(eventID)
	if trimmedEventID == "" {
		return flowTask{}, false, nil
	}
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return flowTask{}, false, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, calendar_event_id, created_at
FROM flow_tasks
WHERE calendar_event_id = ?`, trimmedEventID)
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

func readFlowTaskByIDInTransaction(ctx context.Context, transaction *sql.Tx, taskID string) (flowTask, bool, error) {
	return readFlowTaskByIDWithQueryer(ctx, transaction, taskID)
}

func readFlowTaskByIDWithQueryer(ctx context.Context, queryer flowTaskQueryer, taskID string) (flowTask, bool, error) {
	rows, errorValue := queryer.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, calendar_event_id, created_at
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
	SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, calendar_event_id, created_at
	FROM flow_tasks
	WHERE mattermost_post_id != '' OR id IN (SELECT task_id FROM flow_channel_outbox)
	ORDER BY updated_at DESC`)
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

func (service *Service) readFlowTaskShareActivity(ctx context.Context, startDate string, endDate string) ([]companyShareWorkRow, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT owner_id, owner_name, content, business, type, size, status, start_date, end_date, substr(updated_at, 1, 10)
FROM flow_tasks
WHERE substr(updated_at, 1, 10) >= ? AND substr(updated_at, 1, 10) <= ?
ORDER BY updated_at DESC`, startDate, endDate)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	activity := []companyShareWorkRow{}
	for rows.Next() {
		var row companyShareWorkRow
		if errorValue := rows.Scan(
			&row.MemberID, &row.Name, &row.Title, &row.Business, &row.Type, &row.Size,
			&row.Status, &row.StartDate, &row.EndDate, &row.Date,
		); errorValue != nil {
			return nil, errorValue
		}
		activity = append(activity, row)
	}
	return activity, rows.Err()
}

func (service *Service) readExpiredFlowMattermostPostTaskIDs(ctx context.Context, cutoff time.Time) ([]string, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id
FROM flow_tasks
WHERE mattermost_post_id != '' AND mattermost_post_created_at != '' AND mattermost_post_created_at < ?`,
		cutoff.Format(time.RFC3339))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	taskIDs := []string{}
	for rows.Next() {
		var taskID string
		if errorValue := rows.Scan(&taskID); errorValue != nil {
			return nil, errorValue
		}
		taskIDs = append(taskIDs, strings.TrimSpace(taskID))
	}
	return taskIDs, rows.Err()
}

func readAllFlowTasksInTransaction(ctx context.Context, transaction *sql.Tx) ([]flowTask, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, calendar_event_id, created_at
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

func (service *Service) countFlowTasks(ctx context.Context) (int, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return 0, errorValue
	}
	defer database.Close()
	count := 0
	errorValue = database.QueryRowContext(ctx, "SELECT COUNT(1) FROM flow_tasks").Scan(&count)
	return count, errorValue
}
