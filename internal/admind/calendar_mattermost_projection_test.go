package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCalendarNotificationRemindsTheAuthorWhenNobodyIsNamed(t *testing.T) {
	var directChannelMembers []string
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
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels/direct":
			if errorValue := json.NewDecoder(request.Body).Decode(&directChannelMembers); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusCreated, `{"id":"dm-channel"}`, nil), nil
		case request.Method == http.MethodPut && strings.HasSuffix(request.URL.Path, "/preferences"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/posts":
			assertMattermostBearerToken(t, request, "bot-token")
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			message, _ := payload["message"].(string)
			if strings.Contains(message, "Calendar reminder") {
				postCount++
				postedMessage = message
			}
			return jsonResponse(http.StatusCreated, `{"id":"post-1"}`, nil), nil
		default:
			t.Fatalf("unexpected Mattermost request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})

	event := calendarTestEvent("all-hands", "Company offsite", "Travel prep")
	event.CreatedByName = "샘플"
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	waitForCalendarNotificationReconciliation(t, service, event.ID)
	service.processDueCalendarNotifications(context.Background(), time.Now().UTC().Add(time.Second))
	service.processDueCalendarNotifications(context.Background(), time.Now().UTC().Add(2*time.Second))

	if strings.Join(directChannelMembers, "|") != "user-1|bot-1" {
		t.Fatalf("reminder must reach the author by direct message, got %+v", directChannelMembers)
	}
	if postCount != 1 {
		t.Fatalf("reminder count = %d", postCount)
	}
	if !strings.Contains(postedMessage, "[Company offsite](") || !strings.Contains(postedMessage, "Travel prep") {
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
	waitForCalendarNotificationReconciliation(t, service, event.ID)
	service.processDueCalendarNotifications(context.Background(), time.Now().UTC().Add(time.Second))

	if strings.Join(directChannelMembers, "|") != "user-1|bot-1" {
		t.Fatalf("direct members = %+v; notifications = %s", directChannelMembers, calendarNotificationStateForTest(t, service))
	}
	if !strings.Contains(postedMessage, "[Online sync](") || !strings.Contains(postedMessage, "Bring agenda") || strings.Contains(postedMessage, "샘플") || strings.Contains(postedMessage, "일정 열기") {
		t.Fatalf("posted message = %q", postedMessage)
	}
}

func TestCalendarMattermostLogDirectMessagesEventPeople(t *testing.T) {
	requests := calendarMattermostLogRequests{}
	service := newCalendarMattermostTestService(t, func(request *http.Request) (*http.Response, error) {
		return mattermostCalendarLogLifecycleResponse(t, request, &requests)
	})
	event := calendarTestEvent("logged", "Design review", "김예시\nBring agenda")
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	reloadedEvent, found, errorValue := service.readCalendarEventByID(context.Background(), event.ID)
	if errorValue != nil || !found {
		t.Fatalf("expected reloaded event: found=%v error=%v", found, errorValue)
	}
	if reloadedEvent.MattermostPostID != "" {
		t.Fatalf("a direct message must not be tracked as a channel post, got %q", reloadedEvent.MattermostPostID)
	}
	if len(requests.createdMessages) != 1 || !strings.Contains(requests.createdMessages[0], "[Design review](") || !strings.Contains(requests.createdMessages[0], "event=logged") {
		t.Fatalf("created messages = %+v", requests.createdMessages)
	}
	if len(requests.createTokens) != 1 || requests.createTokens[0] != "Bearer bot-token" {
		t.Fatalf("create tokens = %+v", requests.createTokens)
	}

	reloadedEvent.Title = "Updated review"
	if errorValue := service.writeCalendarEvent(context.Background(), reloadedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(requests.createdMessages) != 2 || !strings.Contains(requests.createdMessages[1], "[Updated review](") {
		t.Fatalf("a change must arrive as a new notice: %+v", requests.createdMessages)
	}
	if len(requests.updatedMessages) != 0 {
		t.Fatalf("a direct notice must not be edited in place, got %+v", requests.updatedMessages)
	}

	if errorValue := service.softDeleteCalendarEvent(context.Background(), reloadedEvent.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(requests.createdMessages) != 3 || !strings.Contains(requests.createdMessages[2], "삭제된 일정") || !strings.Contains(requests.createdMessages[2], "Updated review") {
		t.Fatalf("a removed event must be announced, got %+v", requests.createdMessages)
	}
	if len(requests.deletedPostIDs) != 0 {
		t.Fatalf("nothing is deleted from a channel any more, got %+v", requests.deletedPostIDs)
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
	mattermostUsers := []mattermostUserRecord{{ID: "user-kimyesi", Username: "member2", Nickname: "김예시", Email: "member2@example.com"}}
	message := service.calendarMattermostLogMessageWithUsers(calendarTestEvent("targeted", "Staff sync", "김예시\nBring agenda"), mattermostUsers)
	if !strings.Contains(message, "참석자: @member2\n\n| 일시") {
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
	event := calendarTestEvent("logged", "Design review", "김예시\nBring agenda")
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCalendarProjectionOutboxCount(t, service, 1)

	service.drainCalendarMattermostProjectionOutbox(context.Background())

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

func calendarNotificationStateForTest(t *testing.T, service *Service) string {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		return "calendar database unreadable: " + errorValue.Error()
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(context.Background(),
		"SELECT event_id, recipient_key, target_type, target_value, notify_at, status, error FROM calendar_event_notifications")
	if errorValue != nil {
		return "notification read failed: " + errorValue.Error()
	}
	defer rows.Close()
	states := []string{}
	for rows.Next() {
		var eventID, recipientKey, targetType, targetValue, notifyAt, status, failure string
		if errorValue := rows.Scan(&eventID, &recipientKey, &targetType, &targetValue, &notifyAt, &status, &failure); errorValue != nil {
			return "notification scan failed: " + errorValue.Error()
		}
		states = append(states, fmt.Sprintf("{event:%s recipient:%s target:%s/%s notifyAt:%s status:%s error:%q}",
			eventID, recipientKey, targetType, targetValue, notifyAt, status, failure))
	}
	if len(states) == 0 {
		return "no notification rows were written at all"
	}
	return strings.Join(states, " ")
}
