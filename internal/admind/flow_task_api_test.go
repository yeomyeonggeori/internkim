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

func TestFlowAPICreatesAndPreservesCreatedAt(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	task := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "생성 시각 업무", "진행", 0, []string{staffID}))
	if _, errorValue := time.Parse(time.RFC3339, task.CreatedAt); errorValue != nil {
		t.Fatalf("created at = %q error = %v", task.CreatedAt, errorValue)
	}
	database, errorValue := service.openFlowDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	createdAt := "2026-01-02T03:04:05Z"
	if _, errorValue := database.ExecContext(context.Background(), "UPDATE flow_tasks SET created_at = ? WHERE id = ?", createdAt, task.ID); errorValue != nil {
		t.Fatal(errorValue)
	}

	updatedTask := updateFlowTaskForTest(t, handler, "staff@example.com", task.ID, newFlowTaskPayload("staff@example.com", "수정된 생성 시각 업무", "진행", task.StatusRank, []string{staffID}))
	reloadedTask := readFlowTaskByIDForTest(t, service, task.ID)

	if updatedTask.CreatedAt != createdAt {
		t.Fatalf("updated created at = %q, want %q", updatedTask.CreatedAt, createdAt)
	}
	if reloadedTask.CreatedAt != createdAt {
		t.Fatalf("reloaded created at = %q, want %q", reloadedTask.CreatedAt, createdAt)
	}
}

func TestFlowSchemaCreatesTaskLookupIndexes(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	database, errorValue := service.openFlowDatabase(context.Background())
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

func TestFlowSchemaBackfillsLegacyTaskCreatedAt(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	ctx := context.Background()
	database, errorValue := service.openSQLiteDatabase(ctx, service.Configuration.FlowDatabasePath, nil)
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
		"여명거리",
		"기능",
		"기존 업무",
		"정렬 보존",
		"M",
		"예정",
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

	database, errorValue = service.openFlowDatabase(ctx)
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

func TestFlowAPIAssignsStatusRankAtEndOfStatusColumn(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	firstTask := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "첫 번째 업무", "진행", 0, []string{stableFlowID("staff@example.com")}))
	secondTask := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "두 번째 업무", "진행", 0, []string{stableFlowID("staff@example.com")}))

	if firstTask.StatusRank <= 0 {
		t.Fatalf("first status rank = %d, want positive", firstTask.StatusRank)
	}
	if secondTask.StatusRank <= firstTask.StatusRank {
		t.Fatalf("second status rank = %d, want greater than %d", secondTask.StatusRank, firstTask.StatusRank)
	}
}

func TestFlowAPIAssignsMovedTaskToEndOfTargetStatusColumn(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	targetTask := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "대상 컬럼 기존 업무", "완료", 0, []string{staffID}))
	movedTask := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "이동할 업무", "진행", 0, []string{staffID}))
	payload := newFlowTaskPayload("staff@example.com", movedTask.Content, "완료", movedTask.StatusRank, []string{staffID})

	updatedTask := updateFlowTaskForTest(t, handler, "staff@example.com", movedTask.ID, payload)

	if updatedTask.Status != "완료" {
		t.Fatalf("status = %q, want 완료", updatedTask.Status)
	}
	if updatedTask.StatusRank <= targetTask.StatusRank {
		t.Fatalf("moved status rank = %d, want greater than %d", updatedTask.StatusRank, targetTask.StatusRank)
	}
}

func TestFlowAPIBoardMoveToCompletedSetsEndDate(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	task := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "완료로 이동할 업무", "진행", 0, []string{staffID}))
	if task.EndDate != "" {
		t.Fatalf("initial end date = %q, want empty", task.EndDate)
	}

	response := moveFlowTaskOnBoardResponseForTest(t, handler, "staff@example.com", task.ID, "완료", nil)

	if response.Code != http.StatusOK {
		t.Fatalf("move status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask := readFlowTaskByIDForTest(t, service, task.ID)
	now := flowDateNow()
	expectedEndDate := now.Format("2006-01-02")
	if reloadedTask.Status != "완료" {
		t.Fatalf("status = %q, want 완료", reloadedTask.Status)
	}
	if reloadedTask.EndDate != expectedEndDate {
		t.Fatalf("end date = %q, want %q", reloadedTask.EndDate, expectedEndDate)
	}
	if reloadedTask.WeekCode != weekCodeForFlowDate(expectedEndDate, now) {
		t.Fatalf("week code = %q, want %q", reloadedTask.WeekCode, weekCodeForFlowDate(expectedEndDate, now))
	}
}

func TestFlowAPIPersistsStatusRankForSameStatusUpdate(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	task := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "순서 변경 업무", "진행", 0, []string{staffID}))
	payload := newFlowTaskPayload("staff@example.com", task.Content, "진행", task.StatusRank+2048, []string{staffID})

	updatedTask := updateFlowTaskForTest(t, handler, "staff@example.com", task.ID, payload)

	if updatedTask.StatusRank != task.StatusRank+2048 {
		t.Fatalf("status rank = %d, want %d", updatedTask.StatusRank, task.StatusRank+2048)
	}
}

func TestFlowAPIPersistsExplicitZeroStatusRankForSameStatusUpdate(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	task := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "첫 번째 이동 업무", "진행", 0, []string{staffID}))
	payload := newFlowTaskPayload("staff@example.com", task.Content, "진행", 0, []string{staffID})

	updatedTask := updateFlowTaskForTest(t, handler, "staff@example.com", task.ID, payload)

	if updatedTask.StatusRank != 0 {
		t.Fatalf("status rank = %d, want 0", updatedTask.StatusRank)
	}
}

func TestFlowAPIMovesBoardTaskWithNormalizedRanksInOneRequest(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	firstTask := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "첫 번째 진행 업무", "진행", 0, []string{staffID}))
	secondTask := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "두 번째 진행 업무", "진행", 0, []string{staffID}))
	movedTask := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "이동할 요청 업무", "요청", 0, []string{staffID}))
	firstTask = updateFlowTaskForTest(t, handler, "staff@example.com", firstTask.ID, newFlowTaskPayload("staff@example.com", firstTask.Content, "진행", 1, []string{staffID}))
	secondTask = updateFlowTaskForTest(t, handler, "staff@example.com", secondTask.ID, newFlowTaskPayload("staff@example.com", secondTask.Content, "진행", 2, []string{staffID}))

	response := moveFlowTaskOnBoardResponseForTest(t, handler, "staff@example.com", movedTask.ID, "진행", &secondTask.ID)

	if response.Code != http.StatusOK {
		t.Fatalf("move status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedFirstTask := readFlowTaskByIDForTest(t, service, firstTask.ID)
	reloadedMovedTask := readFlowTaskByIDForTest(t, service, movedTask.ID)
	reloadedSecondTask := readFlowTaskByIDForTest(t, service, secondTask.ID)
	if reloadedMovedTask.Status != "진행" {
		t.Fatalf("moved status = %q, want 진행", reloadedMovedTask.Status)
	}
	if reloadedFirstTask.StatusRank != 1024 || reloadedMovedTask.StatusRank != 2048 || reloadedSecondTask.StatusRank != 3072 {
		t.Fatalf("status ranks = [%d %d %d], want [1024 2048 3072]", reloadedFirstTask.StatusRank, reloadedMovedTask.StatusRank, reloadedSecondTask.StatusRank)
	}
}

func TestFlowAPIMovesBoardTaskWithoutQueueingNeighborRankProjections(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	firstTask := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "첫 번째 진행 업무", "진행", 0, []string{staffID}))
	secondTask := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "두 번째 진행 업무", "진행", 0, []string{staffID}))
	movedTask := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "이동할 요청 업무", "요청", 0, []string{staffID}))
	firstTask = updateFlowTaskForTest(t, handler, "staff@example.com", firstTask.ID, newFlowTaskPayload("staff@example.com", firstTask.Content, "진행", 1, []string{staffID}))
	secondTask = updateFlowTaskForTest(t, handler, "staff@example.com", secondTask.ID, newFlowTaskPayload("staff@example.com", secondTask.Content, "진행", 2, []string{staffID}))
	clearFlowChannelOutboxForTest(t, service)

	response := moveFlowTaskOnBoardResponseForTest(t, handler, "staff@example.com", movedTask.ID, "진행", &secondTask.ID)

	if response.Code != http.StatusOK {
		t.Fatalf("move status = %d body = %s", response.Code, response.Body.String())
	}
	taskIDs, errorValue := service.pendingFlowMattermostProjectionTaskIDs(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(taskIDs) != 0 {
		t.Fatalf("flow projection outbox task ids = %#v, want empty", taskIDs)
	}
}

func TestFlowAPIRejectsBoardTaskMoveFromUnrelatedUser(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	ownerID := stableFlowID("other@example.com")
	task := createFlowTaskForTest(t, handler, "other@example.com", newFlowTaskPayload("other@example.com", "타인 보드 업무", "요청", 0, []string{ownerID}))

	response := moveFlowTaskOnBoardResponseForTest(t, handler, "staff@example.com", task.ID, "진행", nil)

	if response.Code != http.StatusForbidden {
		t.Fatalf("move status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask := readFlowTaskByIDForTest(t, service, task.ID)
	if reloadedTask.Status != "요청" {
		t.Fatalf("status = %q, want 요청", reloadedTask.Status)
	}
}

func TestFlowAPIRejectsBoardTaskMoveWhenTransactionAuthorizationFails(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	otherID := stableFlowID("other@example.com")
	task := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "권한 재확인 업무", "요청", 0, []string{staffID}))
	task.OwnerID = otherID
	task.OwnerName = "other@example.com"
	task.ParticipantIDs = []string{otherID}
	task.ParticipantNames = []string{"other@example.com"}
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}

	_, errorValue := service.writeFlowTaskBoardMove(context.Background(), flowTaskBoardMoveRequest{
		TaskID:       task.ID,
		TargetStatus: "진행",
	}, func(transactionTask flowTask) bool {
		return containsString(transactionTask.ParticipantIDs, staffID)
	})

	if !errors.Is(errorValue, errFlowTaskBoardMoveForbidden) {
		t.Fatalf("move error = %v, want %v", errorValue, errFlowTaskBoardMoveForbidden)
	}
	reloadedTask := readFlowTaskByIDForTest(t, service, task.ID)
	if reloadedTask.Status != "요청" {
		t.Fatalf("status = %q, want 요청", reloadedTask.Status)
	}
}

func TestFlowAPIRejectsBoardTaskMoveToNonBoardStatus(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	task := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "기각 이동 제한 업무", "요청", 0, []string{staffID}))

	response := moveFlowTaskOnBoardResponseForTest(t, handler, "staff@example.com", task.ID, "기각", nil)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("move status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask := readFlowTaskByIDForTest(t, service, task.ID)
	if reloadedTask.Status != "요청" {
		t.Fatalf("status = %q, want 요청", reloadedTask.Status)
	}
}

func TestFlowAPIRejectsBoardTaskMoveBeforeMissingTask(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	task := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "누락 before 이동 제한 업무", "요청", 0, []string{staffID}))
	missingTaskID := "missing-before-task"

	response := moveFlowTaskOnBoardResponseForTest(t, handler, "staff@example.com", task.ID, "진행", &missingTaskID)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("move status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask := readFlowTaskByIDForTest(t, service, task.ID)
	if reloadedTask.Status != "요청" {
		t.Fatalf("status = %q, want 요청", reloadedTask.Status)
	}
}

func TestFlowAPIRejectsNegativeStatusRank(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	task := createFlowTaskForTest(t, handler, "staff@example.com", newFlowTaskPayload("staff@example.com", "잘못된 순서 업무", "진행", 0, []string{staffID}))
	payload := newFlowTaskPayload("staff@example.com", task.Content, "진행", -1, []string{staffID})

	response := updateFlowTaskResponseForTest(t, handler, "staff@example.com", task.ID, payload)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("update status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask, found, errorValue := service.readFlowTaskByID(context.Background(), task.ID)
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

func TestFlowAPIAllowsParticipantToUpdateTask(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	participantID := stableFlowID("staff@example.com")
	task := createFlowTaskForTest(t, handler, "other@example.com", newFlowTaskPayload("other@example.com", "공동 업무", "진행", 0, []string{stableFlowID("other@example.com"), participantID}))
	payload := newFlowTaskPayload("other@example.com", task.Content, "일시정지", task.StatusRank, []string{stableFlowID("other@example.com"), participantID})

	updatedTask := updateFlowTaskForTest(t, handler, "staff@example.com", task.ID, payload)

	if updatedTask.Status != "일시정지" {
		t.Fatalf("status = %q, want 일시정지", updatedTask.Status)
	}
}

func TestFlowAPIRejectsParticipantOwnerChange(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	ownerID := stableFlowID("other@example.com")
	participantID := stableFlowID("staff@example.com")
	task := createFlowTaskForTest(t, handler, "other@example.com", newFlowTaskPayload("other@example.com", "담당자 변경 제한 업무", "진행", 0, []string{ownerID, participantID}))
	payload := newFlowTaskPayload("staff@example.com", task.Content, task.Status, task.StatusRank, []string{participantID})

	response := updateFlowTaskResponseForTest(t, handler, "staff@example.com", task.ID, payload)

	if response.Code != http.StatusForbidden {
		t.Fatalf("update status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask, found, errorValue := service.readFlowTaskByID(context.Background(), task.ID)
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

func TestFlowAPIRejectsParticipantListChange(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	ownerID := stableFlowID("other@example.com")
	participantID := stableFlowID("staff@example.com")
	adminID := stableFlowID("admin@example.com")
	task := createFlowTaskForTest(t, handler, "other@example.com", newFlowTaskPayload("other@example.com", "참여자 변경 제한 업무", "진행", 0, []string{ownerID, participantID}))
	payload := newFlowTaskPayload("other@example.com", task.Content, task.Status, task.StatusRank, []string{ownerID, participantID, adminID})

	response := updateFlowTaskResponseForTest(t, handler, "staff@example.com", task.ID, payload)

	if response.Code != http.StatusForbidden {
		t.Fatalf("update status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask, found, errorValue := service.readFlowTaskByID(context.Background(), task.ID)
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

func TestFlowAPIAllowsOwnerToChangeParticipantList(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	ownerID := stableFlowID("other@example.com")
	participantID := stableFlowID("staff@example.com")
	adminID := stableFlowID("admin@example.com")
	task := createFlowTaskForTest(t, handler, "other@example.com", newFlowTaskPayload("other@example.com", "담당자 참여자 변경 업무", "진행", 0, []string{ownerID, participantID}))
	payload := newFlowTaskPayload("other@example.com", task.Content, task.Status, task.StatusRank, []string{ownerID, adminID})

	updatedTask := updateFlowTaskForTest(t, handler, "other@example.com", task.ID, payload)

	if !sameStringSet(updatedTask.ParticipantIDs, []string{ownerID, adminID}) {
		t.Fatalf("participant ids = %#v, want owner and admin", updatedTask.ParticipantIDs)
	}
}

func TestFlowAPIAllowsAdminToChangeOwner(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	ownerID := stableFlowID("other@example.com")
	adminID := stableFlowID("admin@example.com")
	task := createFlowTaskForTest(t, handler, "other@example.com", newFlowTaskPayload("other@example.com", "관리자 담당자 변경 업무", "진행", 0, []string{ownerID}))
	payload := newFlowTaskPayload("admin@example.com", task.Content, task.Status, task.StatusRank, []string{adminID})

	updatedTask := updateFlowTaskForTest(t, handler, "admin@example.com", task.ID, payload)

	if updatedTask.OwnerID != adminID {
		t.Fatalf("owner id = %q, want %q", updatedTask.OwnerID, adminID)
	}
}

func TestFlowAPIRejectsUpdatingUnrelatedTask(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	task := createFlowTaskForTest(t, handler, "other@example.com", newFlowTaskPayload("other@example.com", "타인 업무", "진행", 0, []string{stableFlowID("other@example.com")}))
	payload := newFlowTaskPayload("other@example.com", task.Content, "완료", task.StatusRank, []string{stableFlowID("other@example.com")})

	response := updateFlowTaskResponseForTest(t, handler, "staff@example.com", task.ID, payload)

	if response.Code != http.StatusForbidden {
		t.Fatalf("update status = %d body = %s", response.Code, response.Body.String())
	}
	reloadedTask, found, errorValue := service.readFlowTaskByID(context.Background(), task.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("expected task to remain")
	}
	if reloadedTask.Status != "진행" {
		t.Fatalf("status = %q, want 진행", reloadedTask.Status)
	}
}

func newFlowTaskPayload(ownerEmail string, content string, status string, statusRank int, participantIDs []string) flowTaskWriteRequest {
	return flowTaskWriteRequest{
		OwnerID:        stableFlowID(ownerEmail),
		ParticipantIDs: participantIDs,
		Type:           "회의",
		Content:        content,
		Size:           "XS",
		Status:         status,
		StatusRank:     &statusRank,
		WeekCode:       "26W18",
	}
}

func createFlowTaskForTest(t *testing.T, handler http.Handler, callerEmail string, payload flowTaskWriteRequest) flowTask {
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
	var task flowTask
	if errorValue := json.NewDecoder(response.Body).Decode(&task); errorValue != nil {
		t.Fatal(errorValue)
	}
	return task
}

func updateFlowTaskForTest(t *testing.T, handler http.Handler, callerEmail string, taskID string, payload flowTaskWriteRequest) flowTask {
	t.Helper()
	response := updateFlowTaskResponseForTest(t, handler, callerEmail, taskID, payload)
	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", response.Code, response.Body.String())
	}
	var task flowTask
	if errorValue := json.NewDecoder(response.Body).Decode(&task); errorValue != nil {
		t.Fatal(errorValue)
	}
	return task
}

func updateFlowTaskResponseForTest(t *testing.T, handler http.Handler, callerEmail string, taskID string, payload flowTaskWriteRequest) *httptest.ResponseRecorder {
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

func moveFlowTaskOnBoardResponseForTest(t *testing.T, handler http.Handler, callerEmail string, taskID string, targetStatus string, beforeTaskID *string) *httptest.ResponseRecorder {
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

func readFlowTaskByIDForTest(t *testing.T, service *Service, taskID string) flowTask {
	t.Helper()
	task, found, errorValue := service.readFlowTaskByID(context.Background(), taskID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatalf("expected task %q to exist", taskID)
	}
	return task
}

func clearFlowChannelOutboxForTest(t *testing.T, service *Service) {
	t.Helper()
	database, errorValue := service.openFlowDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(context.Background(), "DELETE FROM flow_channel_outbox"); errorValue != nil {
		t.Fatal(errorValue)
	}
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
