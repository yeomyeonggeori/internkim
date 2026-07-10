package admind

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCalendarParticipantsFromMembersIncludesImagePath(t *testing.T) {
	participants := calendarParticipantsFromMembers([]flowMember{
		{ID: "person-dongha", Name: "이샘플", Email: "dongha@example.com"},
	})

	if len(participants) != 1 || participants[0].Image != "/calendar/api/participants/person-dongha/image" {
		t.Fatalf("participants = %+v", participants)
	}
}

func TestCalendarParticipantsRejectCalendarToken(t *testing.T) {
	service := newCalendarTestService(t)
	if _, errorValue := service.writeCalendarICSToken(context.Background(), "calendar-token"); errorValue != nil {
		t.Fatal(errorValue)
	}

	tokenRequest := httptest.NewRequest(http.MethodGet, "/calendar/api/participants", nil)
	tokenRequest.RemoteAddr = "203.0.113.10:49152"
	tokenRequest.SetBasicAuth(calendarDAVUsername, "calendar-token")
	tokenResponse := httptest.NewRecorder()
	service.router().ServeHTTP(tokenResponse, tokenRequest)
	if tokenResponse.Code != http.StatusForbidden {
		t.Fatalf("token status = %d body = %s", tokenResponse.Code, tokenResponse.Body.String())
	}

	staffRequest := httptest.NewRequest(http.MethodGet, "/calendar/api/participants", nil)
	staffRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	staffResponse := httptest.NewRecorder()
	service.router().ServeHTTP(staffResponse, staffRequest)
	if staffResponse.Code != http.StatusOK {
		t.Fatalf("staff status = %d body = %s", staffResponse.Code, staffResponse.Body.String())
	}
}

func TestCalendarEventsWithParticipantImagesRestoresCurrentMemberImage(t *testing.T) {
	service := newCalendarTestService(t)
	service.Configuration.APIBaseURL = "http://internkim.local"
	service.Configuration.FleetIDPath = writeTestFile(t, "fleet-1")
	service.Configuration.FleetSecretPath = writeTestFile(t, "fleet-secret")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "http://internkim.local/api/users?fleet_id=fleet-1" {
			return jsonResponse(http.StatusOK, `{"records":[{"email":"dongha@example.com","name":"이샘플","handle":"dongha","role":"member"}]}`, nil), nil
		}
		return jsonResponse(http.StatusNotFound, `{}`, nil), nil
	})}
	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events", nil)
	event := calendarEvent{
		Participants: []calendarParticipant{{PersonID: stableFlowID("dongha@example.com"), Name: "이샘플", Email: "dongha@example.com"}},
	}

	events := service.calendarEventsWithParticipantImages(request, []calendarEvent{event})

	expectedImage := "/calendar/api/participants/" + url.PathEscape(stableFlowID("dongha@example.com")) + "/image"
	if len(events) != 1 || len(events[0].Participants) != 1 || events[0].Participants[0].Image != expectedImage {
		t.Fatalf("events = %+v", events)
	}
}

func TestCalendarNoopUpdateKeepsParticipantImage(t *testing.T) {
	service := newCalendarTestService(t)
	service.Configuration.APIBaseURL = "http://internkim.local"
	service.Configuration.FleetIDPath = writeTestFile(t, "fleet-1")
	service.Configuration.FleetSecretPath = writeTestFile(t, "fleet-secret")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "http://internkim.local/api/users?fleet_id=fleet-1" {
			return jsonResponse(http.StatusOK, `{"records":[{"email":"dongha@example.com","name":"이샘플","handle":"dongha","role":"member"}]}`, nil), nil
		}
		return jsonResponse(http.StatusNotFound, `{}`, nil), nil
	})}
	startTime := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	endTime := startTime.Add(time.Hour)
	personID := stableFlowID("dongha@example.com")
	event := calendarEvent{
		ID:          "noop-participant-image",
		UID:         "noop-participant-image@internkim",
		Title:       "Noop participant image",
		Description: "Bring agenda",
		StartISO:    startTime.Format(time.RFC3339),
		EndISO:      endTime.Format(time.RFC3339),
		TimeZone:    "UTC",
		Color:       "#2563eb",
		Participants: []calendarParticipant{
			{PersonID: personID, Name: "이샘플", Email: "dongha@example.com"},
		},
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
	}
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	updatePayload := `{"title":"Noop participant image","description":"Bring agenda","startISO":"` + event.StartISO + `","endISO":"` + event.EndISO + `","timeZone":"UTC","color":"#2563eb","participants":[{"personID":"` + personID + `","name":"이샘플","email":"dongha@example.com"}],"reminderLeadHours":24}`
	request := httptest.NewRequest(http.MethodPut, "/calendar/api/events/"+event.ID, strings.NewReader(updatePayload))
	request.RemoteAddr = "127.0.0.1:49152"
	request.Header.Set("Content-Type", "application/json")
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var responseEvent calendarEvent
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &responseEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedImage := "/calendar/api/participants/" + url.PathEscape(personID) + "/image"
	if len(responseEvent.Participants) != 1 || responseEvent.Participants[0].Image != expectedImage {
		t.Fatalf("participants = %+v", responseEvent.Participants)
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
			return jsonResponse(http.StatusOK, `{"records":[{"email":"dongha@example.com","name":"이샘플","handle":"dongha","role":"member"}]}`, nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/username/dongha":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"dongha@example.com","username":"dongha","nickname":"이샘플","last_picture_update":1710000000000}`, nil), nil
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
	personID := stableFlowID("dongha@example.com")
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
			return jsonResponse(http.StatusOK, `{"records":[{"email":"dongha@example.com","name":"이샘플","handle":"dongha","role":"member"}]}`, nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/username/dongha":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"dongha@example.com","username":"dongha","nickname":"이샘플","last_picture_update":0}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/user-1/image":
			t.Fatalf("default Mattermost image should not be requested")
			return nil, nil
		}
		return jsonResponse(http.StatusNotFound, `{}`, nil), nil
	})}
	personID := stableFlowID("dongha@example.com")
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/calendar/api/participants/"+url.PathEscape(personID)+"/image", nil)
	request.RemoteAddr = "127.0.0.1:49152"
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("image response = %d %q", responseRecorder.Code, responseRecorder.Body.String())
	}
}
