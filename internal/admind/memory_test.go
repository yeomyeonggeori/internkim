package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
