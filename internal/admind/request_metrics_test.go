package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyBlueclawTaskListPreservesTotalCountResponse(t *testing.T) {
	blueclawServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("method = %s", request.Method)
		}
		if request.URL.String() != "/admin/api/task?limit=15&offset=15&includeTotal=true" {
			t.Fatalf("blueclaw task URL = %s", request.URL.String())
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"taskRuns":[{"taskRunID":"task-1","status":"completed"}],"totalCount":60}`))
	}))
	t.Cleanup(blueclawServer.Close)

	service := NewService(Configuration{BlueclawBaseURL: blueclawServer.URL})
	request := httptest.NewRequest(http.MethodGet, "/admin/api/diagnostics/tasks?limit=15&offset=15&includeTotal=true", nil)
	responseRecorder := httptest.NewRecorder()

	service.proxyBlueclawTaskList(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var body map[string]any
	if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&body); errorValue != nil {
		t.Fatal(errorValue)
	}
	if body["totalCount"] != float64(60) {
		t.Fatalf("totalCount = %#v", body["totalCount"])
	}
	taskRuns, ok := body["taskRuns"].([]any)
	if !ok || len(taskRuns) != 1 {
		t.Fatalf("taskRuns = %#v", body["taskRuns"])
	}
}

func TestProxyScopedTaskListForwardsPaginationQuery(t *testing.T) {
	blueclawServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("method = %s", request.Method)
		}
		expectedURL := "/admin/api/task?dailyCostTaskRunLimit=500&includeCost=true&includeTotal=true&limit=15&offset=15&viewerEmail=staff%40example.com&viewerIsAdmin=false"
		if request.URL.String() != expectedURL {
			t.Fatalf("blueclaw task URL = %s", request.URL.String())
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"taskRuns":[{"taskRunID":"task-16","status":"completed"}],"totalCount":60}`))
	}))
	t.Cleanup(blueclawServer.Close)

	service := NewService(Configuration{BlueclawBaseURL: blueclawServer.URL})
	request := httptest.NewRequest(http.MethodGet, "/tasks/api/runs?limit=15&offset=15&includeTotal=true&includeCost=true&dailyCostTaskRunLimit=500", nil)
	responseRecorder := httptest.NewRecorder()

	service.proxyScopedTaskList(responseRecorder, request, "staff@example.com", false)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var body map[string]any
	if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&body); errorValue != nil {
		t.Fatal(errorValue)
	}
	if body["totalCount"] != float64(60) {
		t.Fatalf("totalCount = %#v", body["totalCount"])
	}
}

func TestProxyScopedTaskDeleteForwardsViewerContext(t *testing.T) {
	blueclawServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("method = %s", request.Method)
		}
		if request.URL.String() != "/admin/api/task/delete" {
			t.Fatalf("blueclaw task delete URL = %s", request.URL.String())
		}
		var body map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil {
			t.Fatal(errorValue)
		}
		if body["taskRunID"] != "task-1" || body["viewerEmail"] != "staff@example.com" || body["viewerIsAdmin"] != true {
			t.Fatalf("unexpected delete body = %#v", body)
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"status":"deleted","taskRunID":"task-1"}`))
	}))
	t.Cleanup(blueclawServer.Close)

	service := NewService(Configuration{BlueclawBaseURL: blueclawServer.URL})
	request := httptest.NewRequest(http.MethodDelete, "/tasks/api/runs/task-1", nil)
	responseRecorder := httptest.NewRecorder()

	service.proxyScopedTaskDelete(responseRecorder, request, "staff@example.com", true)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var body map[string]any
	if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&body); errorValue != nil {
		t.Fatal(errorValue)
	}
	if body["status"] != "deleted" || body["taskRunID"] != "task-1" {
		t.Fatalf("unexpected response = %#v", body)
	}
}




