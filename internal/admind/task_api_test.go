package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTaskAPICreatesAndPreservesCreatedAt(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	memberID := stableTaskID("member@example.com")
	task := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "생성 시각 업무", "in_progress", 0, []string{memberID}))
	if _, errorValue := time.Parse(time.RFC3339, task.CreatedAt); errorValue != nil {
		t.Fatalf("created at = %q error = %v", task.CreatedAt, errorValue)
	}
	database, errorValue := service.openTaskDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	createdAt := "2026-01-02T03:04:05Z"
	if _, errorValue := database.ExecContext(context.Background(), "UPDATE flow_tasks SET created_at = ? WHERE id = ?", createdAt, task.ID); errorValue != nil {
		t.Fatal(errorValue)
	}

	updatedTask := updateTaskForTest(t, handler, "member@example.com", task.ID, newTaskPayload("member@example.com", "수정된 생성 시각 업무", "in_progress", task.StatusRank, []string{memberID}))
	reloadedTask := readTaskByIDForTest(t, service, task.ID)

	if updatedTask.CreatedAt != createdAt {
		t.Fatalf("updated created at = %q, want %q", updatedTask.CreatedAt, createdAt)
	}
	if reloadedTask.CreatedAt != createdAt {
		t.Fatalf("reloaded created at = %q, want %q", reloadedTask.CreatedAt, createdAt)
	}
}

func TestTaskSchemaCreatesTaskLookupIndexes(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	database, errorValue := service.openTaskDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(context.Background(), "PRAGMA index_list(flow_tasks)")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	indexes := map[string]bool{}
	for rows.Next() {
		var sequence int
		var name string
		var unique int
		var origin string
		var partial int
		if errorValue := rows.Scan(&sequence, &name, &unique, &origin, &partial); errorValue != nil {
			t.Fatal(errorValue)
		}
		indexes[name] = true
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, name := range []string{"flow_tasks_week_code_idx", "flow_tasks_start_date_idx", "flow_tasks_end_date_idx"} {
		if !indexes[name] {
			t.Fatalf("missing flow task index %q in %+v", name, indexes)
		}
	}
}

func TestTaskSchemaBackfillsLegacyTaskCreatedAt(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	ctx := context.Background()
	database, errorValue := service.openSQLiteDatabase(ctx, service.stateDatabasePath(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE flow_tasks (
	id TEXT PRIMARY KEY,
	week_code TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	owner_name TEXT NOT NULL,
	participant_ids TEXT NOT NULL,
	participant_names TEXT NOT NULL,
	business TEXT NOT NULL,
	type TEXT NOT NULL,
	content TEXT NOT NULL,
	goal TEXT NOT NULL,
	size TEXT NOT NULL,
	status TEXT NOT NULL,
	start_date TEXT NOT NULL,
	end_date TEXT NOT NULL,
	flag INTEGER NOT NULL,
	request_reason TEXT NOT NULL,
	decision_reason TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	updatedAt := "2026-06-01T12:34:56Z"
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO flow_tasks (
	id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, start_date, end_date, flag, request_reason, decision_reason, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"legacy-task",
		"26W23",
		"member-1",
		"김철수",
		`["member-1"]`,
		`["김철수"]`,
		"샘플거리",
		"기능",
		"기존 업무",
		"정렬 보존",
		"M",
		"planned",
		"2026-06-01",
		"",
		0,
		"",
		"",
		updatedAt,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	database, errorValue = service.openTaskDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var createdAt string
	if errorValue := database.QueryRowContext(ctx, "SELECT created_at FROM flow_tasks WHERE id = ?", "legacy-task").Scan(&createdAt); errorValue != nil {
		t.Fatal(errorValue)
	}
	if createdAt != updatedAt {
		t.Fatalf("created at = %q, want %q", createdAt, updatedAt)
	}
}

func TestTaskAPIAssignsStatusRankAtEndOfStatusColumn(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	firstTask := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "첫 번째 업무", "in_progress", 0, []string{stableTaskID("member@example.com")}))
	secondTask := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "두 번째 업무", "in_progress", 0, []string{stableTaskID("member@example.com")}))

	if firstTask.StatusRank <= 0 {
		t.Fatalf("first status rank = %d, want positive", firstTask.StatusRank)
	}
	if secondTask.StatusRank <= firstTask.StatusRank {
		t.Fatalf("second status rank = %d, want greater than %d", secondTask.StatusRank, firstTask.StatusRank)
	}
}

func TestTaskAPIAssignsMovedTaskToEndOfTargetStatusColumn(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	memberID := stableTaskID("member@example.com")
	targetTask := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "대상 컬럼 기존 업무", "completed", 0, []string{memberID}))
	movedTask := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "이동할 업무", "in_progress", 0, []string{memberID}))
	payload := newTaskPayload("member@example.com", movedTask.Content, "completed", movedTask.StatusRank, []string{memberID})

	updatedTask := updateTaskForTest(t, handler, "member@example.com", movedTask.ID, payload)

	if updatedTask.Status != "completed" {
		t.Fatalf("status = %q, want 완료", updatedTask.Status)
	}
	if updatedTask.StatusRank <= targetTask.StatusRank {
		t.Fatalf("moved status rank = %d, want greater than %d", updatedTask.StatusRank, targetTask.StatusRank)
	}
}

func TestTaskAPIBoardMoveToCompletedSetsEndDate(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	memberID := stableTaskID("member@example.com")
	task := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "완료로 이동할 업무", "in_progress", 0, []string{memberID}))
	if task.EndDate != "" {
		t.Fatalf("initial end date = %q, want empty", task.EndDate)
	}

	response := moveTaskOnBoardResponseForTest(t, handler, "member@example.com", task.ID, "completed", nil)

	if response.Code != http.StatusOK {
		t.Fatalf("move status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask := readTaskByIDForTest(t, service, task.ID)
	now := taskDateNow()
	expectedEndDate := now.Format("2006-01-02")
	if reloadedTask.Status != "completed" {
		t.Fatalf("status = %q, want 완료", reloadedTask.Status)
	}
	if reloadedTask.EndDate != expectedEndDate {
		t.Fatalf("end date = %q, want %q", reloadedTask.EndDate, expectedEndDate)
	}
	if reloadedTask.WeekCode != weekCodeForTaskDate(expectedEndDate, now) {
		t.Fatalf("week code = %q, want %q", reloadedTask.WeekCode, weekCodeForTaskDate(expectedEndDate, now))
	}
}

func TestTaskAPIPersistsStatusRankForSameStatusUpdate(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	memberID := stableTaskID("member@example.com")
	task := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "순서 변경 업무", "in_progress", 0, []string{memberID}))
	payload := newTaskPayload("member@example.com", task.Content, "in_progress", task.StatusRank+2048, []string{memberID})

	updatedTask := updateTaskForTest(t, handler, "member@example.com", task.ID, payload)

	if updatedTask.StatusRank != task.StatusRank+2048 {
		t.Fatalf("status rank = %d, want %d", updatedTask.StatusRank, task.StatusRank+2048)
	}
}

func TestTaskAPIPersistsExplicitZeroStatusRankForSameStatusUpdate(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	memberID := stableTaskID("member@example.com")
	task := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "첫 번째 이동 업무", "in_progress", 0, []string{memberID}))
	payload := newTaskPayload("member@example.com", task.Content, "in_progress", 0, []string{memberID})

	updatedTask := updateTaskForTest(t, handler, "member@example.com", task.ID, payload)

	if updatedTask.StatusRank != 0 {
		t.Fatalf("status rank = %d, want 0", updatedTask.StatusRank)
	}
}

func TestTaskAPIMovesBoardTaskWithNormalizedRanksInOneRequest(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	memberID := stableTaskID("member@example.com")
	firstTask := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "첫 번째 진행 업무", "in_progress", 0, []string{memberID}))
	secondTask := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "두 번째 진행 업무", "in_progress", 0, []string{memberID}))
	movedTask := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "이동할 요청 업무", "requested", 0, []string{memberID}))
	firstTask = updateTaskForTest(t, handler, "member@example.com", firstTask.ID, newTaskPayload("member@example.com", firstTask.Content, "in_progress", 1, []string{memberID}))
	secondTask = updateTaskForTest(t, handler, "member@example.com", secondTask.ID, newTaskPayload("member@example.com", secondTask.Content, "in_progress", 2, []string{memberID}))

	response := moveTaskOnBoardResponseForTest(t, handler, "member@example.com", movedTask.ID, "in_progress", &secondTask.ID)

	if response.Code != http.StatusOK {
		t.Fatalf("move status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedFirstTask := readTaskByIDForTest(t, service, firstTask.ID)
	reloadedMovedTask := readTaskByIDForTest(t, service, movedTask.ID)
	reloadedSecondTask := readTaskByIDForTest(t, service, secondTask.ID)
	if reloadedMovedTask.Status != "in_progress" {
		t.Fatalf("moved status = %q, want 진행", reloadedMovedTask.Status)
	}
	if reloadedFirstTask.StatusRank != 1024 || reloadedMovedTask.StatusRank != 2048 || reloadedSecondTask.StatusRank != 3072 {
		t.Fatalf("status ranks = [%d %d %d], want [1024 2048 3072]", reloadedFirstTask.StatusRank, reloadedMovedTask.StatusRank, reloadedSecondTask.StatusRank)
	}
}

func TestTaskAPIRejectsBoardTaskMoveFromUnrelatedUser(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	ownerID := stableTaskID("other@example.com")
	task := createTaskForTest(t, handler, "other@example.com", newTaskPayload("other@example.com", "타인 보드 업무", "requested", 0, []string{ownerID}))

	response := moveTaskOnBoardResponseForTest(t, handler, "member@example.com", task.ID, "in_progress", nil)

	if response.Code != http.StatusForbidden {
		t.Fatalf("move status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask := readTaskByIDForTest(t, service, task.ID)
	if reloadedTask.Status != "requested" {
		t.Fatalf("status = %q, want 요청", reloadedTask.Status)
	}
}

func TestTaskAPIRejectsBoardTaskMoveWhenTransactionAuthorizationFails(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	memberID := stableTaskID("member@example.com")
	otherID := stableTaskID("other@example.com")
	task := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "권한 재확인 업무", "requested", 0, []string{memberID}))
	task.OwnerID = otherID
	task.OwnerName = "other@example.com"
	task.ParticipantIDs = []string{otherID}
	task.ParticipantNames = []string{"other@example.com"}
	if errorValue := service.writeTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}

	_, errorValue := service.writeTaskBoardMove(context.Background(), taskBoardMoveRequest{
		TaskID:       task.ID,
		TargetStatus: "in_progress",
	}, func(transactionTask Task) bool {
		return containsString(transactionTask.ParticipantIDs, memberID)
	})

	if !errors.Is(errorValue, errTaskBoardMoveForbidden) {
		t.Fatalf("move error = %v, want %v", errorValue, errTaskBoardMoveForbidden)
	}
	reloadedTask := readTaskByIDForTest(t, service, task.ID)
	if reloadedTask.Status != "requested" {
		t.Fatalf("status = %q, want 요청", reloadedTask.Status)
	}
}

func TestTaskAPIRejectsBoardTaskMoveToNonBoardStatus(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	memberID := stableTaskID("member@example.com")
	task := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "기각 이동 제한 업무", "requested", 0, []string{memberID}))

	response := moveTaskOnBoardResponseForTest(t, handler, "member@example.com", task.ID, "rejected", nil)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("move status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask := readTaskByIDForTest(t, service, task.ID)
	if reloadedTask.Status != "requested" {
		t.Fatalf("status = %q, want 요청", reloadedTask.Status)
	}
}

func TestTaskAPIRejectsBoardTaskMoveBeforeMissingTask(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	memberID := stableTaskID("member@example.com")
	task := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "누락 before 이동 제한 업무", "requested", 0, []string{memberID}))
	missingTaskID := "missing-before-task"

	response := moveTaskOnBoardResponseForTest(t, handler, "member@example.com", task.ID, "in_progress", &missingTaskID)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("move status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask := readTaskByIDForTest(t, service, task.ID)
	if reloadedTask.Status != "requested" {
		t.Fatalf("status = %q, want 요청", reloadedTask.Status)
	}
}

func TestTaskAPIRejectsNegativeStatusRank(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	memberID := stableTaskID("member@example.com")
	task := createTaskForTest(t, handler, "member@example.com", newTaskPayload("member@example.com", "잘못된 순서 업무", "in_progress", 0, []string{memberID}))
	payload := newTaskPayload("member@example.com", task.Content, "in_progress", -1, []string{memberID})

	response := updateTaskResponseForTest(t, handler, "member@example.com", task.ID, payload)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("update status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask, found, errorValue := service.readTaskByID(context.Background(), task.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("expected task to remain")
	}
	if reloadedTask.StatusRank != task.StatusRank {
		t.Fatalf("status rank = %d, want %d", reloadedTask.StatusRank, task.StatusRank)
	}
}

func TestTaskAPIAllowsParticipantToUpdateTask(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	participantID := stableTaskID("member@example.com")
	task := createTaskForTest(t, handler, "other@example.com", newTaskPayload("other@example.com", "공동 업무", "in_progress", 0, []string{stableTaskID("other@example.com"), participantID}))
	payload := newTaskPayload("other@example.com", task.Content, "paused", task.StatusRank, []string{stableTaskID("other@example.com"), participantID})

	updatedTask := updateTaskForTest(t, handler, "member@example.com", task.ID, payload)

	if updatedTask.Status != "paused" {
		t.Fatalf("status = %q, want 일시정지", updatedTask.Status)
	}
}

func TestTaskAPIRejectsParticipantOwnerChange(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	ownerID := stableTaskID("other@example.com")
	participantID := stableTaskID("member@example.com")
	task := createTaskForTest(t, handler, "other@example.com", newTaskPayload("other@example.com", "담당자 변경 제한 업무", "in_progress", 0, []string{ownerID, participantID}))
	payload := newTaskPayload("member@example.com", task.Content, task.Status, task.StatusRank, []string{participantID})

	response := updateTaskResponseForTest(t, handler, "member@example.com", task.ID, payload)

	if response.Code != http.StatusForbidden {
		t.Fatalf("update status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask, found, errorValue := service.readTaskByID(context.Background(), task.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("expected task to remain")
	}
	if reloadedTask.OwnerID != ownerID {
		t.Fatalf("owner id = %q, want %q", reloadedTask.OwnerID, ownerID)
	}
	if !sameStringSet(reloadedTask.ParticipantIDs, []string{ownerID, participantID}) {
		t.Fatalf("participant ids = %#v, want owner and participant", reloadedTask.ParticipantIDs)
	}
}

func TestTaskAPIRejectsParticipantListChange(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	ownerID := stableTaskID("other@example.com")
	participantID := stableTaskID("member@example.com")
	adminID := stableTaskID("admin@example.com")
	task := createTaskForTest(t, handler, "other@example.com", newTaskPayload("other@example.com", "참여자 변경 제한 업무", "in_progress", 0, []string{ownerID, participantID}))
	payload := newTaskPayload("other@example.com", task.Content, task.Status, task.StatusRank, []string{ownerID, participantID, adminID})

	response := updateTaskResponseForTest(t, handler, "member@example.com", task.ID, payload)

	if response.Code != http.StatusForbidden {
		t.Fatalf("update status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask, found, errorValue := service.readTaskByID(context.Background(), task.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("expected task to remain")
	}
	if !sameStringSet(reloadedTask.ParticipantIDs, []string{ownerID, participantID}) {
		t.Fatalf("participant ids = %#v, want original participants", reloadedTask.ParticipantIDs)
	}
}

func TestTaskAPIAllowsOwnerToChangeParticipantList(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	ownerID := stableTaskID("other@example.com")
	participantID := stableTaskID("member@example.com")
	adminID := stableTaskID("admin@example.com")
	task := createTaskForTest(t, handler, "other@example.com", newTaskPayload("other@example.com", "담당자 참여자 변경 업무", "in_progress", 0, []string{ownerID, participantID}))
	payload := newTaskPayload("other@example.com", task.Content, task.Status, task.StatusRank, []string{ownerID, adminID})

	updatedTask := updateTaskForTest(t, handler, "other@example.com", task.ID, payload)

	if !sameStringSet(updatedTask.ParticipantIDs, []string{ownerID, adminID}) {
		t.Fatalf("participant ids = %#v, want owner and admin", updatedTask.ParticipantIDs)
	}
}

func TestTaskAPIAllowsAdminToChangeOwner(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	ownerID := stableTaskID("other@example.com")
	adminID := stableTaskID("admin@example.com")
	task := createTaskForTest(t, handler, "other@example.com", newTaskPayload("other@example.com", "관리자 담당자 변경 업무", "in_progress", 0, []string{ownerID}))
	payload := newTaskPayload("admin@example.com", task.Content, task.Status, task.StatusRank, []string{adminID})

	updatedTask := updateTaskForTest(t, handler, "admin@example.com", task.ID, payload)

	if updatedTask.OwnerID != adminID {
		t.Fatalf("owner id = %q, want %q", updatedTask.OwnerID, adminID)
	}
}

func TestTaskAPIRejectsUpdatingUnrelatedTask(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	task := createTaskForTest(t, handler, "other@example.com", newTaskPayload("other@example.com", "타인 업무", "in_progress", 0, []string{stableTaskID("other@example.com")}))
	payload := newTaskPayload("other@example.com", task.Content, "completed", task.StatusRank, []string{stableTaskID("other@example.com")})

	response := updateTaskResponseForTest(t, handler, "member@example.com", task.ID, payload)

	if response.Code != http.StatusForbidden {
		t.Fatalf("update status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask, found, errorValue := service.readTaskByID(context.Background(), task.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("expected task to remain")
	}
	if reloadedTask.Status != "in_progress" {
		t.Fatalf("status = %q, want 진행", reloadedTask.Status)
	}
}

func newTaskPayload(ownerEmail string, content string, status string, statusRank int, participantIDs []string) taskWriteRequest {
	return taskWriteRequest{
		OwnerID:        stableTaskID(ownerEmail),
		ParticipantIDs: participantIDs,
		Type:           "회의",
		Content:        content,
		Size:           "XS",
		Status:         status,
		StatusRank:     &statusRank,
		WeekCode:       "26W18",
	}
}

func createTaskForTest(t *testing.T, handler http.Handler, callerEmail string, payload taskWriteRequest) Task {
	t.Helper()
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/flow/api/tasks", bytes.NewReader(document))
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", callerEmail)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", response.Code, response.Body.String())
	}
	var task Task
	if errorValue := json.NewDecoder(response.Body).Decode(&task); errorValue != nil {
		t.Fatal(errorValue)
	}
	return task
}

func updateTaskForTest(t *testing.T, handler http.Handler, callerEmail string, taskID string, payload taskWriteRequest) Task {
	t.Helper()
	response := updateTaskResponseForTest(t, handler, callerEmail, taskID, payload)
	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", response.Code, response.Body.String())
	}
	var task Task
	if errorValue := json.NewDecoder(response.Body).Decode(&task); errorValue != nil {
		t.Fatal(errorValue)
	}
	return task
}

func updateTaskResponseForTest(t *testing.T, handler http.Handler, callerEmail string, taskID string, payload taskWriteRequest) *httptest.ResponseRecorder {
	t.Helper()
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPut, "/flow/api/tasks/"+taskID, bytes.NewReader(document))
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", callerEmail)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func moveTaskOnBoardResponseForTest(t *testing.T, handler http.Handler, callerEmail string, taskID string, targetStatus string, beforeTaskID *string) *httptest.ResponseRecorder {
	t.Helper()
	document, errorValue := json.Marshal(map[string]any{
		"taskID":       taskID,
		"targetStatus": targetStatus,
		"beforeTaskID": beforeTaskID,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/flow/api/tasks/move", bytes.NewReader(document))
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", callerEmail)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func readTaskByIDForTest(t *testing.T, service *Service, taskID string) Task {
	t.Helper()
	task, found, errorValue := service.readTaskByID(context.Background(), taskID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatalf("expected task %q to exist", taskID)
	}
	return task
}

func sameStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := map[string]int{}
	for _, value := range left {
		seen[value]++
	}
	for _, value := range right {
		seen[value]--
		if seen[value] < 0 {
			return false
		}
	}
	return true
}
