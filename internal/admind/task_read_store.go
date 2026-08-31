package admind

import (
	"context"
	"database/sql"
	"strings"
)

type taskQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (service *Service) readTasks(ctx context.Context, weekCode string, members []taskMember) ([]Task, error) {
	if tasks, answered, errorValue := service.companyBoardTasks(ctx, members); answered {
		if errorValue != nil {
			return nil, errorValue
		}
		return tasksInWeek(tasks, weekCode), nil
	}
	database, errorValue := service.openTaskDatabase(ctx)
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
	tasks := []Task{}
	for rows.Next() {
		task, errorValue := scanTask(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		tasks = append(tasks, task)
	}
	return alignTasksWithMembers(tasks, members), rows.Err()
}

func (service *Service) readAllTasks(ctx context.Context, members []taskMember) ([]Task, error) {
	if tasks, answered, errorValue := service.companyBoardTasks(ctx, members); answered {
		if errorValue != nil {
			return nil, errorValue
		}
		return tasks, nil
	}
	database, errorValue := service.openTaskDatabase(ctx)
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
	tasks := []Task{}
	for rows.Next() {
		task, errorValue := scanTask(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		tasks = append(tasks, task)
	}
	return alignTasksWithMembers(tasks, members), rows.Err()
}

func (service *Service) readTasksBetweenDates(ctx context.Context, startDate string, endDate string, members []taskMember) ([]Task, error) {
	if tasks, answered, errorValue := service.companyBoardTasks(ctx, members); answered {
		if errorValue != nil {
			return nil, errorValue
		}
		return tasksBetweenDays(tasks, startDate, endDate), nil
	}
	database, errorValue := service.openTaskDatabase(ctx)
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
	tasks := []Task{}
	for rows.Next() {
		task, errorValue := scanTask(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		tasks = append(tasks, task)
	}
	return alignTasksWithMembers(tasks, members), rows.Err()
}

// The company board answers first: a local row answering for a company task
// carries the device's own identifier, and a save keyed by that identifier
// creates a second company task instead of updating the one it meant. The
// device store answers only devices that have no company.
func (service *Service) readTaskAnswering(ctx context.Context, taskID string, members []taskMember) (Task, bool, error) {
	boardTasks, answered, boardError := service.companyBoardTasks(ctx, members)
	if !answered {
		return service.readTaskByID(ctx, taskID)
	}
	if boardError != nil {
		return Task{}, false, boardError
	}
	for _, boardTask := range boardTasks {
		if boardTask.ID == strings.TrimSpace(taskID) {
			return boardTask, true, nil
		}
	}
	return Task{}, false, nil
}

func (service *Service) readTaskByID(ctx context.Context, taskID string) (Task, bool, error) {
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return Task{}, false, errorValue
	}
	defer database.Close()
	return readTaskByIDWithQueryer(ctx, database, taskID)
}

func (service *Service) readTaskByCalendarEventID(ctx context.Context, eventID string) (Task, bool, error) {
	trimmedEventID := strings.TrimSpace(eventID)
	if trimmedEventID == "" {
		return Task{}, false, nil
	}
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return Task{}, false, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, calendar_event_id, created_at
FROM flow_tasks
WHERE calendar_event_id = ?`, trimmedEventID)
	if errorValue != nil {
		return Task{}, false, errorValue
	}
	defer rows.Close()
	if !rows.Next() {
		return Task{}, false, rows.Err()
	}
	task, errorValue := scanTask(rows)
	if errorValue != nil {
		return Task{}, false, errorValue
	}
	return task, true, rows.Err()
}

func readTaskByIDInTransaction(ctx context.Context, transaction *sql.Tx, taskID string) (Task, bool, error) {
	return readTaskByIDWithQueryer(ctx, transaction, taskID)
}

func readTaskByIDWithQueryer(ctx context.Context, queryer taskQueryer, taskID string) (Task, bool, error) {
	rows, errorValue := queryer.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, calendar_event_id, created_at
FROM flow_tasks
WHERE id = ?`, taskID)
	if errorValue != nil {
		return Task{}, false, errorValue
	}
	defer rows.Close()
	if !rows.Next() {
		return Task{}, false, rows.Err()
	}
	task, errorValue := scanTask(rows)
	if errorValue != nil {
		return Task{}, false, errorValue
	}
	return task, true, rows.Err()
}

func (service *Service) readTaskShareActivity(ctx context.Context, startDate string, endDate string) ([]companyShareWorkRow, error) {
	database, errorValue := service.openTaskDatabase(ctx)
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

func readAllTasksInTransaction(ctx context.Context, transaction *sql.Tx) ([]Task, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, calendar_event_id, created_at
FROM flow_tasks`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		task, errorValue := scanTask(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}
