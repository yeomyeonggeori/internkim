package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCalendarNotificationPostsAnnouncementsForAllHands(t *testing.T) {
	var createdChannel map[string]any
	var postedMessage string
	postCount := 0
	service := newCalendarMattermostTestService(t, func(request *http.Request) (*http.Response, error) {
		if response, handled := mattermostCalendarLogTestResponse(t, request); handled {
			return response, nil
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users":
			return jsonResponse(http.StatusOK, `[{"id":"user-1","username":"gamyeong","nickname":"샘플","email":"gamyeong@example.com"}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/teams/team-1/channels/name/announcements":
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels":
			if errorValue := json.NewDecoder(request.Body).Decode(&createdChannel); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusCreated, `{"id":"announcements-channel"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users/username/internkim":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels/announcements-channel/members":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/posts":
			assertMattermostBearerToken(t, request, "bot-token")
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			postCount++
			postedMessage, _ = payload["message"].(string)
			return jsonResponse(http.StatusCreated, `{"id":"post-1"}`, nil), nil
		default:
			t.Fatalf("unexpected Mattermost request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})

	event := calendarTestEvent("all-hands", "Company offsite", "Travel prep")
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.processDueCalendarNotifications(context.Background(), time.Now().UTC().Add(time.Second))
	service.processDueCalendarNotifications(context.Background(), time.Now().UTC().Add(2*time.Second))

	if createdChannel["name"] != calendarAnnouncementsChannelName || createdChannel["display_name"] != announcementsChannelDisplayName(workspaceLanguageKorean) {
		t.Fatalf("created channel = %+v", createdChannel)
	}
	if postCount != 1 {
		t.Fatalf("postCount = %d", postCount)
	}
	if !strings.Contains(postedMessage, "[Company offsite](") || !strings.Contains(postedMessage, "event=all-hands") || !strings.Contains(postedMessage, "Travel prep") || strings.Contains(postedMessage, "일정 열기") {
		t.Fatalf("posted message = %q", postedMessage)
	}
}

func TestCalendarNotificationPostsDirectMessageForPeopleLine(t *testing.T) {
	var directChannelMembers []string
	var postedMessage string
	service := newCalendarMattermostTestService(t, func(request *http.Request) (*http.Response, error) {
		if response, handled := mattermostCalendarLogTestResponse(t, request); handled {
			return response, nil
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users":
			return jsonResponse(http.StatusOK, `[{"id":"user-1","username":"gamyeong","nickname":"샘플","email":"gamyeong@example.com"}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users/username/internkim":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels/direct":
			if errorValue := json.NewDecoder(request.Body).Decode(&directChannelMembers); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusCreated, `{"id":"dm-channel"}`, nil), nil
		case request.Method == http.MethodPut && request.URL.Path == "/api/v4/users/user-1/preferences":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/posts":
			assertMattermostBearerToken(t, request, "bot-token")
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			postedMessage, _ = payload["message"].(string)
			return jsonResponse(http.StatusCreated, `{"id":"post-1"}`, nil), nil
		default:
			t.Fatalf("unexpected Mattermost request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})

	event := calendarTestEvent("targeted", "Online sync", "샘플\nBring agenda")
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.processDueCalendarNotifications(context.Background(), time.Now().UTC().Add(time.Second))

	if strings.Join(directChannelMembers, "|") != "user-1|bot-1" {
		t.Fatalf("direct members = %+v", directChannelMembers)
	}
	if !strings.Contains(postedMessage, "[Online sync](") || !strings.Contains(postedMessage, "Bring agenda") || strings.Contains(postedMessage, "샘플") || strings.Contains(postedMessage, "일정 열기") {
		t.Fatalf("posted message = %q", postedMessage)
	}
}

func TestCalendarMattermostLogCreatesUpdatesAndDeletesPost(t *testing.T) {
	requests := calendarMattermostLogRequests{}
	service := newCalendarMattermostTestService(t, func(request *http.Request) (*http.Response, error) {
		return mattermostCalendarLogLifecycleResponse(t, request, &requests)
	})
	event := calendarTestEvent("logged", "Design review", "김표본\nBring agenda")
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	reloadedEvent, found, errorValue := service.readCalendarEventByID(context.Background(), event.ID)
	if errorValue != nil || !found {
		t.Fatalf("expected reloaded event: found=%v error=%v", found, errorValue)
	}
	if reloadedEvent.MattermostPostID != "calendar-post-1" {
		t.Fatalf("post id = %q", reloadedEvent.MattermostPostID)
	}
	if len(requests.createdMessages) != 1 || !strings.Contains(requests.createdMessages[0], "[Design review](") || !strings.Contains(requests.createdMessages[0], "event=logged") || !strings.Contains(requests.createdMessages[0], "참석자: @iam\n\n| 일시") || strings.Contains(requests.createdMessages[0], "대상:") || strings.Contains(requests.createdMessages[0], "일정 열기") {
		t.Fatalf("created messages = %+v", requests.createdMessages)
	}
	if len(requests.createTokens) != 1 || requests.createTokens[0] != "Bearer bot-token" {
		t.Fatalf("create tokens = %+v", requests.createTokens)
	}
	reloadedEvent.Title = "Updated review"
	if errorValue := service.writeCalendarEvent(context.Background(), reloadedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(requests.updatedMessages) != 1 || !strings.Contains(requests.updatedMessages[0], "[Updated review](") || !strings.Contains(requests.updatedMessages[0], "event=logged") || strings.Contains(requests.updatedMessages[0], "일정 열기") {
		t.Fatalf("updated messages = %+v", requests.updatedMessages)
	}
	if len(requests.updateTokens) != 1 || requests.updateTokens[0] != "Bearer bot-token" {
		t.Fatalf("update tokens = %+v", requests.updateTokens)
	}
	if errorValue := service.softDeleteCalendarEvent(context.Background(), reloadedEvent.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(requests.deletedPostIDs) != 1 || requests.deletedPostIDs[0] != "calendar-post-1" {
		t.Fatalf("deleted posts = %+v", requests.deletedPostIDs)
	}
	projection, found, errorValue := service.readCalendarEventProjectionByID(context.Background(), reloadedEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("expected deleted projection: found=%v error=%v", found, errorValue)
	}
	if !projection.IsDeleted || projection.Event.MattermostPostID != "" {
		t.Fatalf("deleted projection = %+v", projection)
	}
	eventIDs, errorValue := service.readCalendarEventIDsRequiringMattermostProjection(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if slices.Contains(eventIDs, reloadedEvent.ID) {
		t.Fatalf("deleted event remained in projection queue: %v", eventIDs)
	}
}

func TestCalendarMattermostLogOmitsMentionWhenPeopleAreEmpty(t *testing.T) {
	service := newCalendarTestService(t)
	message := service.calendarMattermostLogMessage(calendarTestEvent("all-hands", "Company offsite", "Travel prep"))
	if strings.Contains(message, "@all") {
		t.Fatalf("event with no attendees must not tag everyone; message = %q", message)
	}
}

func TestCalendarMattermostLogMentionsCircleIDPeople(t *testing.T) {
	service := newCalendarTestService(t)
	message := service.calendarMattermostLogMessage(calendarTestEvent("staff-sync", "Staff sync", "staff, product-team\nBring agenda"))
	if !strings.Contains(message, "참석자: @staff @product-team\n\n| 일시") {
		t.Fatalf("message = %q", message)
	}
	if strings.Contains(message, "대상:") {
		t.Fatalf("message = %q", message)
	}
}

func TestCalendarMattermostLogMentionsKoreanPeople(t *testing.T) {
	service := newCalendarTestService(t)
	mattermostUsers := []mattermostUserRecord{{ID: "user-iam", Username: "iam", Nickname: "김표본", Email: "iam@example.com"}}
	message := service.calendarMattermostLogMessageWithUsers(calendarTestEvent("targeted", "Staff sync", "김표본\nBring agenda"), mattermostUsers)
	if !strings.Contains(message, "참석자: @iam\n\n| 일시") {
		t.Fatalf("message = %q", message)
	}
	if strings.Contains(message, "대상:") {
		t.Fatalf("message = %q", message)
	}
}

func TestCalendarMattermostProjectionOutboxRetriesFailedCreate(t *testing.T) {
	createAttempts := 0
	service := newCalendarMattermostTestService(t, func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/posts":
			createAttempts++
			if createAttempts == 1 {
				return jsonResponse(http.StatusServiceUnavailable, `{}`, nil), nil
			}
			return jsonResponse(http.StatusCreated, `{"id":"calendar-post-1"}`, nil), nil
		default:
			return mattermostCalendarLogLifecycleResponse(t, request, &calendarMattermostLogRequests{})
		}
	})
	event := calendarTestEvent("logged", "Design review", "샘플\nBring agenda")
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	reloadedEvent, found, errorValue := service.readCalendarEventByID(context.Background(), event.ID)
	if errorValue != nil || !found {
		t.Fatalf("expected reloaded event: found=%v error=%v", found, errorValue)
	}
	if reloadedEvent.MattermostPostID != "" {
		t.Fatalf("post id after failed projection = %q", reloadedEvent.MattermostPostID)
	}
	assertCalendarProjectionOutboxCount(t, service, 1)

	service.drainCalendarMattermostProjectionOutbox(context.Background())

	reloadedEvent, found, errorValue = service.readCalendarEventByID(context.Background(), event.ID)
	if errorValue != nil || !found {
		t.Fatalf("expected reloaded event: found=%v error=%v", found, errorValue)
	}
	if reloadedEvent.MattermostPostID != "calendar-post-1" {
		t.Fatalf("post id after reconcile = %q", reloadedEvent.MattermostPostID)
	}
	if createAttempts != 2 {
		t.Fatalf("create attempts = %d", createAttempts)
	}
	assertCalendarProjectionOutboxCount(t, service, 0)
}

func TestCalendarNotificationCancelsWhenEventIsDeleted(t *testing.T) {
	service := newCalendarTestService(t)
	event := calendarTestEvent("cancel-me", "Canceled meeting", "")
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(context.Background(), event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	waitForCalendarNotificationReconciliation(t, service, event.ID)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var status string
	row := database.QueryRowContext(context.Background(), "SELECT status FROM calendar_event_notifications WHERE event_id = ?", event.ID)
	if errorValue := row.Scan(&status); errorValue != nil {
		t.Fatal(errorValue)
	}
	if status != "canceled" {
		t.Fatalf("status = %q", status)
	}
}
