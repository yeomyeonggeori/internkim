package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const taskCarryRecoveryAction = "task-carry-into-the-record"
const taskCarryTimeout = 10 * time.Second

// A device that names no company covers nothing, so it counts everything it
// holds as uncovered and keeps its store.
func (service *Service) sweepTheTasksTheCompanyNowHolds(ctx context.Context) {
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return
	}
	defer database.Close()

	if errorValue := dropTaskTables(ctx, database, derivedTaskTables); errorValue != nil {
		slog.WarnContext(ctx, "a derived task table could not be dropped", "error", errorValue)
		return
	}

	pinned := taskTablesTheRecordCannotTake(ctx, database)
	if len(pinned) > 0 {
		slog.WarnContext(ctx, "this device holds tasks the record has no place for, so its store stays",
			"tables", strings.Join(pinned, ","), "recovery_action", taskCarryRecoveryAction)
		return
	}

	uncovered, errorValue := service.tasksTheRecordDoesNotHold(ctx, database)
	if errorValue != nil {
		slog.WarnContext(ctx, "the tasks this device still holds could not be counted, so none of them were let go",
			"error", errorValue)
		return
	}
	if uncovered > 0 {
		slog.WarnContext(ctx, "this device holds tasks the record does not, so its store stays",
			"uncovered", uncovered, "recovery_action", taskCarryRecoveryAction)
		return
	}
	if errorValue := dropTaskTables(ctx, database, carriedTaskTables); errorValue != nil {
		slog.WarnContext(ctx, "a task table the company now holds could not be dropped", "error", errorValue)
	}
}

func taskTablesTheRecordCannotTake(ctx context.Context, database *sql.DB) []string {
	pinned := []string{}
	for _, tableName := range sortedTaskTableNames() {
		rowCount, held := countRowsInTaskTable(ctx, database, tableName)
		if held && rowCount > 0 {
			pinned = append(pinned, tableName+" ("+taskTablesWithNowhereToGo[tableName]+")")
		}
	}
	return pinned
}

func sortedTaskTableNames() []string {
	names := make([]string, 0, len(taskTablesWithNowhereToGo))
	for tableName := range taskTablesWithNowhereToGo {
		names = append(names, tableName)
	}
	slices.Sort(names)
	return names
}

// Every task this device holds that the record has not taken. A carried row is
// remembered by id, so asking again after a carry answers zero.
func (service *Service) tasksTheRecordDoesNotHold(ctx context.Context, database *sql.DB) (int, error) {
	if service.centralPlane() == nil {
		rowCount, held := countRowsInTaskTable(ctx, database, "flow_tasks")
		if !held {
			return 0, nil
		}
		return rowCount, nil
	}
	tasks, errorValue := service.uncarriedTasks(ctx, database)
	if errorValue != nil {
		return 0, errorValue
	}
	return len(tasks), nil
}

type deviceTask struct {
	ID             string
	ParticipantIDs []string
	Business       string
	Type           string
	Title          string
	Size           string
	Status         string
	StartDate      string
	EndDate        string
}

const deviceTaskQuery = `
SELECT id, participant_ids, business, type, content, size, status, start_date, end_date
FROM flow_tasks
ORDER BY created_at ASC`

func (service *Service) uncarriedTasks(ctx context.Context, database *sql.DB) ([]deviceTask, error) {
	alreadyCarried, errorValue := carriedTaskRowIDs(ctx, database)
	if errorValue != nil {
		return nil, errorValue
	}
	if _, held := countRowsInTaskTable(ctx, database, "flow_tasks"); !held {
		return nil, nil
	}
	rows, errorValue := database.QueryContext(ctx, deviceTaskQuery)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	tasks := []deviceTask{}
	for rows.Next() {
		var task deviceTask
		var participantIDsDocument string
		if errorValue := rows.Scan(&task.ID, &participantIDsDocument, &task.Business, &task.Type,
			&task.Title, &task.Size, &task.Status, &task.StartDate, &task.EndDate); errorValue != nil {
			return nil, errorValue
		}
		if _, carried := alreadyCarried[task.ID]; carried {
			continue
		}
		_ = json.Unmarshal([]byte(participantIDsDocument), &task.ParticipantIDs)
		task.Status = cleanTaskStatus(task.Status)
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

type taskCarryReport struct {
	Tasks   int      `json:"tasks"`
	Refused []string `json:"refused"`
}

// Every task is written as the person it belongs to: the record's insert policy
// asks only that the writer be a colleague, so nothing here needs an
// administrator. A row the record refuses is reported, not reshaped.
func (service *Service) carryTasksIntoTheRecord(ctx context.Context) (taskCarryReport, error) {
	report := taskCarryReport{Refused: []string{}}
	client := service.centralPlane()
	if client == nil {
		return report, fmt.Errorf("this device names no company to carry its tasks into")
	}
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return report, errorValue
	}
	defer database.Close()

	tasks, errorValue := service.uncarriedTasks(ctx, database)
	if errorValue != nil {
		return report, errorValue
	}
	addressByPersonID := service.taskPeopleByID(ctx)
	companyToday := service.companyToday(ctx, client)
	for _, task := range tasks {
		refusal := service.carryOneTask(ctx, client, database, task, addressByPersonID, companyToday)
		if refusal != "" {
			report.Refused = append(report.Refused, task.ID+": "+refusal)
			continue
		}
		report.Tasks++
	}
	return report, nil
}

func (service *Service) carryOneTask(
	ctx context.Context,
	client *centralplane.Client,
	database *sql.DB,
	task deviceTask,
	addressByPersonID map[string]adminUserMutation,
	companyToday time.Time,
) string {
	addresses := carriedTaskAddresses(task, addressByPersonID)
	if len(addresses) == 0 {
		return "it names nobody the company knows"
	}

	carryContext, cancel := context.WithTimeout(ctx, taskCarryTimeout)
	defer cancel()

	heldID, errorValue := client.TaskCarrying(carryContext, "email", addresses[0], task.ID)
	if errorValue != nil {
		return errorValue.Error()
	}
	if strings.TrimSpace(heldID) != "" {
		rememberCarriedTaskRow(ctx, database, task.ID)
		return ""
	}

	if refusal := taskTheRecordWouldReshape(task, companyToday); refusal != "" {
		return refusal
	}

	savedID, errorValue := client.SaveTask(carryContext, centralplane.Task{
		ActorPlatform:    "email",
		ActorExternalID:  addresses[0],
		Title:            task.Title,
		Status:           task.Status,
		Business:         task.Business,
		Type:             task.Type,
		Size:             task.Size,
		StartsAt:         task.StartDate,
		EndsAt:           task.EndDate,
		WritesDates:      task.StartDate != "" || task.EndDate != "",
		ParticipantMails: addresses,
	})
	if errorValue != nil {
		return errorValue.Error()
	}
	if errorValue := client.MarkCarriedFrom(carryContext, "email", addresses[0], savedID, task.ID); errorValue != nil {
		return "the record took it as " + savedID + " but would not mark where it came from: " + errorValue.Error()
	}
	rememberCarriedTaskRow(ctx, database, task.ID)
	return ""
}

// The record accepts a task whose history it cannot represent by quietly
// rewriting it: a completed row with a future or missing end is restamped with
// today, a reversed pair of dates is swapped, and a requested row is given the
// writer as its requester. The device never recorded a requester, so each of
// these is a row this carry refuses rather than invents.
func taskTheRecordWouldReshape(task deviceTask, companyToday time.Time) string {
	if !isAllowedTaskStatus(task.Status) {
		return "status " + task.Status + " is not one the record keeps"
	}
	startsAt, refusal := carriedTaskDay(task.StartDate, "start")
	if refusal != "" {
		return refusal
	}
	endsAt, refusal := carriedTaskDay(task.EndDate, "end")
	if refusal != "" {
		return refusal
	}
	if !startsAt.IsZero() && !endsAt.IsZero() && endsAt.Before(startsAt) {
		return "it ends on " + task.EndDate + ", before it starts on " + task.StartDate
	}
	if task.Status == taskStatusRequested || task.Status == taskStatusRejected {
		return "a " + task.Status + " task names a requester the record demands and this device never recorded (" +
			taskRequesterIssueURL + ")"
	}
	if task.Status != taskStatusCompleted {
		return ""
	}
	if !startsAt.IsZero() && startsAt.After(companyToday) {
		return "it is completed but starts on " + task.StartDate + ", after today"
	}
	if endsAt.IsZero() || endsAt.After(companyToday) {
		return "it is completed but ends on " + firstNonEmpty(task.EndDate, "no day at all") + ", which the record would restamp with today"
	}
	return ""
}

const taskRequesterIssueURL = "https://github.com/yeomyeonggeori/internkim/issues/1457"

func carriedTaskDay(day string, name string) (time.Time, string) {
	trimmed := strings.TrimSpace(day)
	if trimmed == "" {
		return time.Time{}, ""
	}
	parsed, errorValue := time.Parse(time.DateOnly, trimmed)
	if errorValue != nil {
		return time.Time{}, name + " date " + day + " is not a date"
	}
	return parsed, ""
}

func carriedTaskAddresses(task deviceTask, addressByPersonID map[string]adminUserMutation) []string {
	addresses := []string{}
	for _, personID := range task.ParticipantIDs {
		person, known := addressByPersonID[personID]
		if !known || strings.TrimSpace(person.Email) == "" {
			continue
		}
		addresses = append(addresses, person.Email)
	}
	return addresses
}

// The record decides a day boundary in the company's own timezone, so that is
// the one a refusal has to agree with.
func (service *Service) companyToday(ctx context.Context, client *centralplane.Client) time.Time {
	location := taskDateLocation()
	company, found, errorValue := client.Company(ctx)
	if errorValue == nil && found {
		if named, loadError := time.LoadLocation(strings.TrimSpace(company.Timezone)); loadError == nil {
			location = named
		}
	}
	now := time.Now().In(location)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// What the record took, kept against the local id so a second sweep does not
// count a carried row as still missing. The table is retired with the store it
// describes, so it never outlives what it is about.
func carriedTaskRowIDs(ctx context.Context, database *sql.DB) (map[string]struct{}, error) {
	carried := map[string]struct{}{}
	if _, held := countRowsInTaskTable(ctx, database, "task_carried_rows"); !held {
		return carried, nil
	}
	rows, errorValue := database.QueryContext(ctx, "SELECT id FROM task_carried_rows")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if errorValue := rows.Scan(&id); errorValue != nil {
			return nil, errorValue
		}
		carried[id] = struct{}{}
	}
	return carried, rows.Err()
}

func rememberCarriedTaskRow(ctx context.Context, database *sql.DB, id string) {
	if _, errorValue := database.ExecContext(ctx,
		"CREATE TABLE IF NOT EXISTS task_carried_rows (id TEXT PRIMARY KEY)"); errorValue != nil {
		return
	}
	database.ExecContext(ctx, "INSERT OR IGNORE INTO task_carried_rows (id) VALUES (?)", id)
}

type taskRecordCoverage struct {
	Tasks   int      `json:"tasks"`
	Pinned  []string `json:"pinned"`
	Carried int      `json:"carried"`
	Refused []string `json:"refused"`
}

func (service *Service) handleTaskRecordCoverage(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebMemberRequest(request) {
		http.Error(responseWriter, "member access required", http.StatusForbidden)
		return
	}
	coverage, errorValue := service.taskCoverageOfTheRecord(request)
	if errorValue != nil {
		log.Printf("task coverage failed: %v", errorValue)
		http.Error(responseWriter, "task_coverage_failed: "+errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, coverage)
}

func (service *Service) taskCoverageOfTheRecord(request *http.Request) (taskRecordCoverage, error) {
	coverage := taskRecordCoverage{Pinned: []string{}, Refused: []string{}}
	database, errorValue := service.openTaskDatabase(request.Context())
	if errorValue != nil {
		return coverage, errorValue
	}
	tasks, errorValue := service.uncarriedTasks(request.Context(), database)
	if errorValue != nil {
		database.Close()
		return coverage, errorValue
	}
	coverage.Tasks = len(tasks)
	coverage.Pinned = taskTablesTheRecordCannotTake(request.Context(), database)
	database.Close()

	if request.URL.Query().Get("carry") != "true" {
		return coverage, nil
	}
	report, errorValue := service.carryTasksIntoTheRecord(request.Context())
	if errorValue != nil {
		return coverage, errorValue
	}
	coverage.Carried = report.Tasks
	coverage.Refused = report.Refused
	return coverage, nil
}
