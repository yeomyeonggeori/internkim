package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestMemoryAPIUsesMattermostSessionUserGraph(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:        "https://api.intern.kim",
		BlueclawBaseURL:   "http://blueclaw.local",
		FleetIDPath:       writeTestFile(t, "device-1"),
		FleetSecretPath:   writeTestFile(t, "secret-1"),
		MattermostBaseURL: "http://mattermost.local",
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://mattermost.local/api/v4/users/me" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"id":"mattermost-user-1","email":"member@example.com","username":"member"}`, nil), nil
		}
		if request.URL.String() == "https://api.intern.kim/api/users?fleet_id=device-1" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"records":[{"email":"member@example.com","userID":"user:person-1","name":"Member","role":"member","status":"active"}]}`, nil), nil
		}
		if request.URL.Path == "/admin/api/memory/graph" && request.Method == http.MethodGet {
			if request.URL.Query().Get("readerPersonID") != "user:person-1" {
				t.Fatalf("readerPersonID = %q", request.URL.Query().Get("readerPersonID"))
			}
			return jsonResponse(http.StatusOK, `{"nodes":[],"edges":[],"facts":[],"episodes":[],"namespaces":[]}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodGet, "/memory/api/graph?limit=10", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("memory graph status = %d body = %s", response.Code, response.Body.String())
	}
	var graph map[string]any
	if errorValue := json.NewDecoder(response.Body).Decode(&graph); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, ok := graph["nodes"]; !ok {
		t.Fatalf("memory graph response = %+v", graph)
	}
}

func TestMemoryAPIUsesMattermostSessionUserSchedules(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:        "https://api.intern.kim",
		BlueclawBaseURL:   "http://blueclaw.local",
		FleetIDPath:       writeTestFile(t, "device-1"),
		FleetSecretPath:   writeTestFile(t, "secret-1"),
		MattermostBaseURL: "http://mattermost.local",
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://mattermost.local/api/v4/users/me" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"id":"mattermost-user-1","email":"member@example.com","username":"member"}`, nil), nil
		}
		if request.URL.String() == "https://api.intern.kim/api/users?fleet_id=device-1" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"records":[{"email":"member@example.com","userID":"user:person-1","name":"Member","role":"member","status":"active"}]}`, nil), nil
		}
		if request.URL.Path == "/admin/api/policy" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"people":[{"personID":"person-1","emails":["member@example.com"]}]}`, nil), nil
		}
		if request.URL.Path == "/admin/api/task-schedules" && request.Method == http.MethodGet {
			if request.URL.Query().Get("creatorPersonID") != "person-1" {
				t.Fatalf("creatorPersonID = %q", request.URL.Query().Get("creatorPersonID"))
			}
			if request.URL.Query().Get("limit") != "" {
				t.Fatalf("limit = %q", request.URL.Query().Get("limit"))
			}
			if request.URL.Query().Get("page") != "3" {
				t.Fatalf("page = %q", request.URL.Query().Get("page"))
			}
			if request.URL.Query().Get("pageSize") != "25" {
				t.Fatalf("pageSize = %q", request.URL.Query().Get("pageSize"))
			}
			return jsonResponse(http.StatusOK, `{"schedules":[{"taskScheduleID":"schedule-1","creatorPersonID":"person-1","executionMode":"agent","kind":"cron","cronExpression":"0 9 * * *","nextRunAt":"2026-06-09T00:00:00Z","createdAt":"2026-06-08T00:00:00Z","updatedAt":"2026-06-08T00:00:00Z","deliveryChannelID":"channel-1","promptPreview":"팀 일정 알려주기"}],"count":1}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodGet, "/memory/api/schedules?creatorPersonID=other-person&limit=200&page=3&pageSize=25", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("memory schedules status = %d body = %s", response.Code, response.Body.String())
	}
	var schedules map[string]any
	if errorValue := json.NewDecoder(response.Body).Decode(&schedules); errorValue != nil {
		t.Fatal(errorValue)
	}
	if schedules["count"] != float64(1) {
		t.Fatalf("memory schedules response = %+v", schedules)
	}
}

func TestMemorySchedulesQuerySanitizesPagination(t *testing.T) {
	query := memorySchedulesQuery(url.Values{
		"creatorPersonID": []string{"other-person"},
		"limit":           []string{"200"},
		"page":            []string{"0"},
		"pageSize":        []string{"999"},
	}, "person-1")
	values, errorValue := url.ParseQuery(query)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if values.Get("creatorPersonID") != "person-1" {
		t.Fatalf("creatorPersonID = %q", values.Get("creatorPersonID"))
	}
	if values.Get("limit") != "" {
		t.Fatalf("limit = %q", values.Get("limit"))
	}
	if values.Get("page") != "1" {
		t.Fatalf("page = %q", values.Get("page"))
	}
	if values.Get("pageSize") != "100" {
		t.Fatalf("pageSize = %q", values.Get("pageSize"))
	}
}

func TestMemoryAPISchedulesHidesUpstreamFailureDetails(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:        "https://api.intern.kim",
		BlueclawBaseURL:   "http://blueclaw.local",
		FleetIDPath:       writeTestFile(t, "device-1"),
		FleetSecretPath:   writeTestFile(t, "secret-1"),
		MattermostBaseURL: "http://mattermost.local",
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://mattermost.local/api/v4/users/me" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"id":"mattermost-user-1","email":"member@example.com","username":"member"}`, nil), nil
		}
		if request.URL.String() == "https://api.intern.kim/api/users?fleet_id=device-1" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"records":[{"email":"member@example.com","userID":"user:person-1","name":"Member","role":"member","status":"active"}]}`, nil), nil
		}
		if request.URL.Path == "/admin/api/policy" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"people":[{"personID":"person-1","emails":["member@example.com"]}]}`, nil), nil
		}
		if request.URL.Path == "/admin/api/task-schedules" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusInternalServerError, `private backend detail`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodGet, "/memory/api/schedules", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("memory schedules status = %d body = %s", response.Code, response.Body.String())
	}
	responseBody := response.Body.String()
	if strings.Contains(responseBody, "private backend detail") {
		t.Fatalf("memory schedules leaked upstream detail: %s", responseBody)
	}
	if !strings.Contains(responseBody, "memory schedules unavailable") {
		t.Fatalf("memory schedules body = %s", responseBody)
	}
}
