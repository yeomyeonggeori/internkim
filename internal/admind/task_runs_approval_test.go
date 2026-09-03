package admind

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type recordedBlueclaw struct {
	detailQuery   url.Values
	approveBodies []string
}

func blueclawRefusingDetail(recorded *recordedBlueclaw) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/admin/api/run/detail":
			recorded.detailQuery = request.URL.Query()
			http.Error(responseWriter, "task run not found", http.StatusNotFound)
		case "/admin/api/run/approve":
			body, _ := io.ReadAll(request.Body)
			recorded.approveBodies = append(recorded.approveBodies, strings.TrimSpace(string(body)))
			responseWriter.Header().Set("Content-Type", "application/json")
			_, _ = responseWriter.Write([]byte(`{"taskRunID":"run-1","status":"running"}`))
		default:
			http.NotFound(responseWriter, request)
		}
	}))
}

func blueclawAnsweringDetail(recorded *recordedBlueclaw) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/admin/api/run/detail":
			recorded.detailQuery = request.URL.Query()
			responseWriter.Header().Set("Content-Type", "application/json")
			_, _ = responseWriter.Write([]byte(`{"taskRun":{"taskRunID":"run-1","status":"waiting_approval"},"taskEvents":[]}`))
		case "/admin/api/run/approve":
			body, _ := io.ReadAll(request.Body)
			recorded.approveBodies = append(recorded.approveBodies, strings.TrimSpace(string(body)))
			responseWriter.Header().Set("Content-Type", "application/json")
			_, _ = responseWriter.Write([]byte(`{"taskRunID":"run-1","status":"running"}`))
		default:
			http.NotFound(responseWriter, request)
		}
	}))
}

func approvalRequest(document string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/runs/api/approve", strings.NewReader(document))
}

func TestApprovalIsRefusedForARunTheViewerCannotSee(t *testing.T) {
	recorded := &recordedBlueclaw{}
	blueclaw := blueclawRefusingDetail(recorded)
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL})

	recorder := httptest.NewRecorder()
	service.proxyScopedTaskApproval(recorder, approvalRequest(`{"taskRunID":"run-1","decision":"confirm"}`), "member@example.com", false)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("a run the viewer cannot see answered %d, want 404", recorder.Code)
	}
	if len(recorded.approveBodies) != 0 {
		t.Fatalf("the decision reached Blueclaw anyway: %v", recorded.approveBodies)
	}
}

func TestApprovalAsksBlueclawWhetherTheViewerMaySeeTheRunFirst(t *testing.T) {
	recorded := &recordedBlueclaw{}
	blueclaw := blueclawAnsweringDetail(recorded)
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL})

	recorder := httptest.NewRecorder()
	service.proxyScopedTaskApproval(recorder, approvalRequest(`{"taskRunID":"run-1","decision":"confirm_task"}`), "member@example.com", false)

	if recorder.Code != http.StatusOK {
		t.Fatalf("a run the viewer may see answered %d, want 200", recorder.Code)
	}
	if recorded.detailQuery.Get("viewerEmail") != "member@example.com" {
		t.Fatalf("the visibility check carried viewerEmail %q", recorded.detailQuery.Get("viewerEmail"))
	}
	if recorded.detailQuery.Get("viewerIsAdmin") != "false" {
		t.Fatalf("the visibility check carried viewerIsAdmin %q", recorded.detailQuery.Get("viewerIsAdmin"))
	}
	if len(recorded.approveBodies) != 1 {
		t.Fatalf("Blueclaw was asked to decide %d times, want 1", len(recorded.approveBodies))
	}
	var forwarded taskApprovalRequest
	if errorValue := json.Unmarshal([]byte(recorded.approveBodies[0]), &forwarded); errorValue != nil {
		t.Fatalf("the forwarded decision was unreadable: %v", errorValue)
	}
	if forwarded.TaskRunID != "run-1" || forwarded.Decision != "confirm_task" {
		t.Fatalf("the forwarded decision was %+v", forwarded)
	}
}

func TestApprovalCarriesNothingTheCallerDidNotName(t *testing.T) {
	recorded := &recordedBlueclaw{}
	blueclaw := blueclawAnsweringDetail(recorded)
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL})

	recorder := httptest.NewRecorder()
	service.proxyScopedTaskApproval(
		recorder,
		approvalRequest(`{"taskRunID":"run-1","decision":"cancel","viewerIsAdmin":true,"viewerEmail":"admin@example.com"}`),
		"member@example.com",
		false,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("the decision answered %d, want 200", recorder.Code)
	}
	if strings.Contains(recorded.approveBodies[0], "viewerIsAdmin") || strings.Contains(recorded.approveBodies[0], "admin@example.com") {
		t.Fatalf("a caller-supplied viewer reached Blueclaw: %s", recorded.approveBodies[0])
	}
	if recorded.detailQuery.Get("viewerIsAdmin") != "false" {
		t.Fatalf("the caller widened its own scope to %q", recorded.detailQuery.Get("viewerIsAdmin"))
	}
}

func TestApprovalWithoutATaskRunIDIsRefusedBeforeBlueclawIsAsked(t *testing.T) {
	recorded := &recordedBlueclaw{}
	blueclaw := blueclawAnsweringDetail(recorded)
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL})

	recorder := httptest.NewRecorder()
	service.proxyScopedTaskApproval(recorder, approvalRequest(`{"decision":"confirm"}`), "member@example.com", false)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("a decision naming no run answered %d, want 400", recorder.Code)
	}
	if recorded.detailQuery != nil {
		t.Fatal("Blueclaw was asked about a run nobody named")
	}
}

func TestApprovalIsPostOnly(t *testing.T) {
	recorded := &recordedBlueclaw{}
	blueclaw := blueclawAnsweringDetail(recorded)
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL})

	recorder := httptest.NewRecorder()
	service.proxyScopedTaskApproval(recorder, httptest.NewRequest(http.MethodGet, "/runs/api/approve", nil), "member@example.com", false)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("a GET on the approval path answered %d, want 404", recorder.Code)
	}
}

func TestApprovalPathIsRoutedToTheTaskRunHandler(t *testing.T) {
	service := &Service{}
	router := service.router()

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, approvalRequest(`{"taskRunID":"run-1","decision":"confirm"}`))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("an unauthenticated decision answered %d; it is 401", recorder.Code)
	}
}
