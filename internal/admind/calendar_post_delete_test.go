package admind

import (
	"net/http"
	"testing"
)

// Mattermost answers a delete for a post it no longer has with 404 and
// app.post.get.app_error. The calendar projection returned that as a failure, so
// it never reached the line clearing the stored post id and retried the same
// deleted post every minute for as long as the event existed.
func TestDeletingACalendarLogPostThatIsAlreadyGoneSucceeds(t *testing.T) {
	service := NewService(Configuration{
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-pass"),
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/v4/users/login" {
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		}
		return jsonResponse(http.StatusNotFound,
			`{"id":"app.post.get.app_error","message":"Unable to get the post.","status_code":404}`, nil), nil
	})}

	errorValue := service.tryDeleteCalendarMattermostLog(t.Context(), calendarEvent{
		ID:               "event-1",
		MattermostPostID: "a-post-mattermost-no-longer-has",
	})

	if errorValue != nil {
		t.Fatalf("the post is gone, which is what the delete wanted: %v", errorValue)
	}
}

func TestDeletingACalendarLogPostStillFailsWhenMattermostIsBroken(t *testing.T) {
	service := NewService(Configuration{
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-pass"),
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/v4/users/login" {
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		}
		return jsonResponse(http.StatusInternalServerError, `{"id":"app.post.delete.app_error","status_code":500}`, nil), nil
	})}

	if errorValue := service.tryDeleteCalendarMattermostLog(t.Context(), calendarEvent{
		ID:               "event-1",
		MattermostPostID: "a-post-that-is-there",
	}); errorValue == nil {
		t.Fatal("a broken server has to keep the caller retrying")
	}
}
