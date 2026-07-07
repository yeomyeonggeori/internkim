package admind

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestProofOfConceptTaskListRejectsTenantAdmin(t *testing.T) {
	service := newProofOfConceptTaskAccessTestService(t, "admin01@example.test", `{"id":"tenant-admin","email":"admin01@example.test","username":"admin01","roles":"system_user","delete_at":0}`)
	request := httptest.NewRequest(http.MethodGet, "/tasks/api/runs", nil)
	request.RemoteAddr = "203.0.113.10:12345"
	request.Header.Set("X-Forwarded-Email", "admin01@example.test")
	responseRecorder := httptest.NewRecorder()

	service.handleTasks(responseRecorder, request)

	if responseRecorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}

func TestProofOfConceptTaskListAllowsMattermostSystemAdmin(t *testing.T) {
	blueclawServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		expectedURL := "/admin/api/task?includeTotal=true&viewerEmail=admin%40localhost&viewerIsAdmin=true"
		if request.URL.String() != expectedURL {
			t.Fatalf("blueclaw task URL = %s", request.URL.String())
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"taskRuns":[{"taskRunID":"task-1","status":"completed"}],"totalCount":1}`))
	}))
	t.Cleanup(blueclawServer.Close)

	service := newProofOfConceptTaskAccessTestService(t, "admin@localhost", `{"id":"admin","email":"admin@localhost","username":"admin","roles":"system_admin system_user","delete_at":0}`)
	service.Configuration.BlueclawBaseURL = blueclawServer.URL
	request := httptest.NewRequest(http.MethodGet, "/tasks/api/runs?includeTotal=true", nil)
	request.RemoteAddr = "203.0.113.10:12345"
	request.Header.Set("X-Forwarded-Email", "admin@localhost")
	responseRecorder := httptest.NewRecorder()

	service.handleTasks(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var body map[string]any
	if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&body); errorValue != nil {
		t.Fatal(errorValue)
	}
	if body["totalCount"] != float64(1) {
		t.Fatalf("totalCount = %#v", body["totalCount"])
	}
}

func TestProofOfConceptTaskListRejectsSystemAdminBot(t *testing.T) {
	service := newProofOfConceptTaskAccessTestService(t, "internkim01@example.test", `{"id":"bot","email":"internkim01@example.test","username":"internkim01","roles":"system_admin system_user","delete_at":0,"is_bot":true}`)
	request := httptest.NewRequest(http.MethodGet, "/tasks/api/runs", nil)
	request.RemoteAddr = "203.0.113.10:12345"
	request.Header.Set("X-Forwarded-Email", "internkim01@example.test")
	responseRecorder := httptest.NewRecorder()

	service.handleTasks(responseRecorder, request)

	if responseRecorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}

func newProofOfConceptTaskAccessTestService(t *testing.T, email string, userDocument string) *Service {
	t.Helper()
	service := NewService(Configuration{
		MattermostBaseURL:           "http://mattermost.local",
		MattermostTeamName:          "tenant01",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-password"),
		AdminEmailPath:              writeTestFile(t, "admin01@example.test"),
	})
	defaultTransport := http.DefaultTransport
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "mattermost.local" {
			return defaultTransport.RoundTrip(request)
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/api/v4/users/email/"):
			if strings.HasSuffix(request.URL.Path, email) {
				return jsonResponse(http.StatusOK, userDocument, nil), nil
			}
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		default:
			document, _ := io.ReadAll(request.Body)
			t.Fatalf("unexpected Mattermost request %s %s %s", request.Method, request.URL.String(), string(document))
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		}
	})}
	return service
}
