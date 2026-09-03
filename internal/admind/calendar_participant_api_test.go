package admind

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCalendarParticipantsFromMembersIncludesImagePath(t *testing.T) {
	participants := calendarParticipantsFromMembers([]taskMember{
		{ID: "person-gamyeong", Name: "이샘플", Email: "gamyeong@example.com"},
	})

	if len(participants) != 1 || participants[0].Image != "/calendar/api/participants/person-gamyeong/image" {
		t.Fatalf("participants = %+v", participants)
	}
}

func TestCalendarEventsWithParticipantImagesRestoresCurrentMemberImage(t *testing.T) {
	service := newCalendarTestService(t)
	service.Configuration.APIBaseURL = "http://internkim.local"
	service.Configuration.FleetIDPath = writeTestFile(t, "fleet-1")
	service.Configuration.FleetSecretPath = writeTestFile(t, "fleet-secret")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "http://internkim.local/api/users?fleet_id=fleet-1" {
			return jsonResponse(http.StatusOK, `{"records":[{"email":"gamyeong@example.com","name":"이샘플","handle":"gamyeong","role":"member"}]}`, nil), nil
		}
		return jsonResponse(http.StatusNotFound, `{}`, nil), nil
	})}
	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events", nil)
	event := calendarEvent{
		Participants: []calendarParticipant{{PersonID: stableTaskID("gamyeong@example.com"), Name: "이샘플", Email: "gamyeong@example.com"}},
	}

	events := service.calendarEventsWithParticipantImages(request, []calendarEvent{event})

	expectedImage := "/calendar/api/participants/" + url.PathEscape(stableTaskID("gamyeong@example.com")) + "/image"
	if len(events) != 1 || len(events[0].Participants) != 1 || events[0].Participants[0].Image != expectedImage {
		t.Fatalf("events = %+v", events)
	}
}

func TestCalendarParticipantImageServesMattermostImage(t *testing.T) {
	service := newCalendarTestService(t)
	service.Configuration.APIBaseURL = "http://internkim.local"
	service.Configuration.FleetIDPath = writeTestFile(t, "fleet-1")
	service.Configuration.FleetSecretPath = writeTestFile(t, "fleet-secret")
	service.Configuration.MattermostBaseURL = "http://mattermost.local"
	service.Configuration.MattermostAdminPasswordPath = writeTestFile(t, "admin-password")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "http://internkim.local/api/users?fleet_id=fleet-1":
			return jsonResponse(http.StatusOK, `{"records":[{"email":"gamyeong@example.com","name":"이샘플","handle":"gamyeong","role":"member"}]}`, nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/username/gamyeong":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"gamyeong@example.com","username":"gamyeong","nickname":"이샘플","last_picture_update":1710000000000}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/user-1/image":
			assertMattermostBearerToken(t, request, "admin-token")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("profile-image")),
				Header:     http.Header{"Content-Type": []string{"image/png"}},
			}, nil
		}
		return jsonResponse(http.StatusNotFound, `{}`, nil), nil
	})}
	personID := stableTaskID("gamyeong@example.com")
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/calendar/api/participants/"+url.PathEscape(personID)+"/image", nil)
	request.RemoteAddr = "127.0.0.1:49152"
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK || responseRecorder.Body.String() != "profile-image" || responseRecorder.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("image response = %d %q %q", responseRecorder.Code, responseRecorder.Body.String(), responseRecorder.Header().Get("Content-Type"))
	}
}

func TestCalendarParticipantImageFallsBackWhenMattermostProfileImageIsMissing(t *testing.T) {
	service := newCalendarTestService(t)
	service.Configuration.APIBaseURL = "http://internkim.local"
	service.Configuration.FleetIDPath = writeTestFile(t, "fleet-1")
	service.Configuration.FleetSecretPath = writeTestFile(t, "fleet-secret")
	service.Configuration.MattermostBaseURL = "http://mattermost.local"
	service.Configuration.MattermostAdminPasswordPath = writeTestFile(t, "admin-password")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "http://internkim.local/api/users?fleet_id=fleet-1":
			return jsonResponse(http.StatusOK, `{"records":[{"email":"gamyeong@example.com","name":"이샘플","handle":"gamyeong","role":"member"}]}`, nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/username/gamyeong":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"gamyeong@example.com","username":"gamyeong","nickname":"이샘플","last_picture_update":0}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/user-1/image":
			t.Fatalf("default Mattermost image should not be requested")
			return nil, nil
		}
		return jsonResponse(http.StatusNotFound, `{}`, nil), nil
	})}
	personID := stableTaskID("gamyeong@example.com")
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/calendar/api/participants/"+url.PathEscape(personID)+"/image", nil)
	request.RemoteAddr = "127.0.0.1:49152"
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("image response = %d %q", responseRecorder.Code, responseRecorder.Body.String())
	}
}
