package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
