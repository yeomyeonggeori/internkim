package admind

import (
	"context"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestReadFlowTasksWithMattermostPostsSkipsRetiredNotifications(t *testing.T) {
	service := NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	ctx := context.Background()

	postedTask := flowNotificationTestTask("완료")
	postedTask.ID = "posted-task"
	writeFlowTaskWithSettledPost(t, service, postedTask, "posted-post")

	retiredTask := flowNotificationTestTask("완료")
	retiredTask.ID = "retired-task"
	writeFlowTaskWithSettledPost(t, service, retiredTask, "retired-post")
	backdateFlowMattermostPostCreatedAt(t, service, retiredTask.ID, time.Now().UTC().Add(-20*24*time.Hour))
	settleFlowMattermostPost(t, service, retiredTask.ID, "")

	queuedTask := flowNotificationTestTask("요청")
	queuedTask.ID = "queued-task"
	if errorValue := service.writeFlowTask(ctx, queuedTask); errorValue != nil {
		t.Fatal(errorValue)
	}

	tasks, errorValue := service.readFlowTasksWithMattermostPosts(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	taskIDs := []string{}
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	if slices.Contains(taskIDs, retiredTask.ID) {
		t.Fatalf("a retention-retired notification would be posted again on restart: %v", taskIDs)
	}
	if !slices.Contains(taskIDs, postedTask.ID) || !slices.Contains(taskIDs, queuedTask.ID) {
		t.Fatalf("reconcile lost a task that still needs a post: %v", taskIDs)
	}
}

func TestReadCalendarEventIDsRequiringMattermostProjectionSkipsRetiredLogs(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()

	loggedEvent := calendarTestEvent("logged-event", "Design review", "Bring agenda")
	writeCalendarEventWithSettledPost(t, service, loggedEvent, "logged-post")

	retiredEvent := calendarTestEvent("retired-event", "Old retro", "Bring notes")
	writeCalendarEventWithSettledPost(t, service, retiredEvent, "retired-post")
	settleCalendarMattermostPost(t, service, retiredEvent.ID, "")

	eventIDs, errorValue := service.readCalendarEventIDsRequiringMattermostProjection(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if slices.Contains(eventIDs, retiredEvent.ID) {
		t.Fatalf("a retention-retired calendar log would be posted again on restart: %v", eventIDs)
	}
	if !slices.Contains(eventIDs, loggedEvent.ID) {
		t.Fatalf("reconcile lost an event that still has a log post: %v", eventIDs)
	}
}

func writeFlowTaskWithSettledPost(t *testing.T, service *Service, task flowTask, postID string) {
	t.Helper()
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}
	settleFlowMattermostPost(t, service, task.ID, postID)
}

func settleFlowMattermostPost(t *testing.T, service *Service, taskID string, postID string) {
	t.Helper()
	ctx := context.Background()
	if errorValue := service.updateFlowTaskMattermostPostID(ctx, taskID, postID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.deleteFlowMattermostProjectionOutbox(ctx, taskID); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func writeCalendarEventWithSettledPost(t *testing.T, service *Service, event calendarEvent, postID string) {
	t.Helper()
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	settleCalendarMattermostPost(t, service, event.ID, postID)
}

func settleCalendarMattermostPost(t *testing.T, service *Service, eventID string, postID string) {
	t.Helper()
	ctx := context.Background()
	if errorValue := service.updateCalendarEventMattermostPostID(ctx, eventID, postID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.deleteCalendarMattermostProjectionOutbox(ctx, eventID); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestFlowMattermostNotificationRetiresPostThatIsGone(t *testing.T) {
	service, createdPostCount := newRetiredFlowPostTestService(t)
	task := flowNotificationTestTask("완료")
	writeFlowTaskWithSettledPost(t, service, task, "flow-post-1")

	task.MattermostPostID = "flow-post-1"
	task = service.syncFlowMattermostNotification(context.Background(), task)

	if *createdPostCount != 0 {
		t.Fatalf("a card whose post is already gone was posted again %d time(s)", *createdPostCount)
	}
	reloadedTask, found, errorValue := service.readFlowTaskByID(context.Background(), task.ID)
	if errorValue != nil || !found {
		t.Fatalf("expected reloaded task: found=%v error=%v", found, errorValue)
	}
	if reloadedTask.MattermostPostID != "" {
		t.Fatalf("stale post id survived retirement: %q", reloadedTask.MattermostPostID)
	}
}

func TestFlowMattermostNotificationRepostsWhenProjectionIsPending(t *testing.T) {
	service, createdPostCount := newRetiredFlowPostTestService(t)
	task := flowNotificationTestTask("완료")
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.updateFlowTaskMattermostPostID(context.Background(), task.ID, "flow-post-1"); errorValue != nil {
		t.Fatal(errorValue)
	}

	task.MattermostPostID = "flow-post-1"
	task = service.syncFlowMattermostNotification(context.Background(), task)

	if *createdPostCount != 1 {
		t.Fatalf("a task that still owes a projection was posted %d time(s)", *createdPostCount)
	}
	if task.MattermostPostID != "flow-post-2" {
		t.Fatalf("post id = %q", task.MattermostPostID)
	}
}

func newRetiredFlowPostTestService(t *testing.T) (*Service, *int) {
	t.Helper()
	createdPostCount := 0
	service := NewService(Configuration{
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-pass"),
		MattermostBotTokenPath:      writeTestFile(t, "bot-token"),
		FlowDatabasePath:            filepath.Join(t.TempDir(), "flow.sqlite"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/me":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users?per_page=200" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `[]`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/flow-post-1" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusNotFound, `{"status_code":404}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/flow-post-1" && request.Method == http.MethodDelete:
			return jsonResponse(http.StatusNotFound, `{"status_code":404}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
			createdPostCount++
			return jsonResponse(http.StatusCreated, `{"id":"flow-post-2"}`, nil), nil
		case strings.HasSuffix(request.URL.String(), "/members/bot-1/schemeRoles") && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostExistingFlowSetupResponse(t, request), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	return service, &createdPostCount
}
