package admind

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type calendarNotificationTarget struct {
	TargetType string
	Key        string
	Label      string
	UserID     string
}

type calendarNotification struct {
	EventID      string
	RecipientKey string
	TargetType   string
	TargetValue  string
	TargetLabel  string
	NotifyAt     string
	Status       string
}

const calendarNotificationQuietStartHour = 21
const calendarNotificationQuietEndHour = 8

func (service *Service) syncCalendarMattermostLog(ctx context.Context, event calendarEvent) calendarEvent {
	return service.applyCalendarMattermostProjection(ctx, event)
}

func (service *Service) trySyncCalendarMattermostLog(ctx context.Context, event calendarEvent) (calendarEvent, error) {
	if strings.TrimSpace(service.Configuration.MattermostAdminPasswordPath) == "" {
		return event, nil
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return event, errorValue
	}
	botToken, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		return event, errorValue
	}
	mattermostUsers, errorValue := service.activeMattermostUsers(ctx, adminToken)
	if errorValue != nil {
		return event, errorValue
	}
	recipients := calendarEventChangeRecipients(event, mattermostUsers)
	message := service.changeNoticeWording(ctx, service.calendarNoticeFacts(event, "등록되거나 변경됨"), service.calendarMattermostLogMessage(event))
	return event, service.deliverCalendarNotice(ctx, adminToken, botToken, event, recipients, message)
}

func (service *Service) notifyCalendarEventRemoved(ctx context.Context, event calendarEvent) error {
	if strings.TrimSpace(service.Configuration.MattermostAdminPasswordPath) == "" {
		return nil
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	botToken, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		return errorValue
	}
	mattermostUsers, errorValue := service.activeMattermostUsers(ctx, adminToken)
	if errorValue != nil {
		return errorValue
	}
	recipients := calendarEventChangeRecipients(event, mattermostUsers)
	message := service.changeNoticeWording(ctx, service.calendarNoticeFacts(event, "삭제됨"), service.calendarRemovedMessage(event))
	return service.deliverCalendarNotice(ctx, adminToken, botToken, event, recipients, message)
}

func (service *Service) deliverCalendarNotice(ctx context.Context, adminToken string, botToken string, event calendarEvent, recipients []changeRecipient, message string) error {
	noticeKey := changeNoticeKey(event.ID, message)
	delivered, errorValue := service.sendChangeDirectMessages(ctx, adminToken, botToken, recipients, message, calendarMattermostLogProps(event), service.deliveredCalendarRecipients(ctx, event.ID, noticeKey))
	service.recordDeliveredCalendarRecipients(ctx, event.ID, noticeKey, delivered)
	return errorValue
}

func (service *Service) calendarRemovedMessage(event calendarEvent) string {
	workspaceLocation, _ := service.workspaceTimeLocation()
	return mattermostMarkdownTable(
		[]string{"일시", "삭제된 일정", "장소"},
		[][]string{{
			calendarMattermostEventDateText(event, workspaceLocation),
			event.Title,
			event.Location,
		}},
	)
}

func (service *Service) calendarMattermostLogMessage(event calendarEvent) string {
	return service.calendarMattermostLogMessageWithUsers(event, nil)
}

func (service *Service) calendarMattermostLogMessageWithUsers(event calendarEvent, mattermostUsers []mattermostUserRecord) string {
	workspaceLocation, _ := service.workspaceTimeLocation()
	lines := []string{}
	if mentionText := calendarMattermostMentionText(event, mattermostUsers); mentionText != "" {
		lines = append(lines, "참석자: "+mentionText, "")
	}
	lines = append(lines, mattermostMarkdownTable(
		[]string{"일시", "일정", "장소", "메모"},
		[][]string{{
			calendarMattermostEventDateText(event, workspaceLocation),
			mattermostMarkdownLink(event.Title, service.mattermostCalendarEventURL(event)),
			event.Location,
			calendarMattermostNoteText(event.Description),
		}},
	))
	return strings.Join(lines, "\n")
}

func calendarMattermostMentionText(event calendarEvent, mattermostUsers []mattermostUserRecord) string {
	people, hasPeople := calendarNotificationPeople(event)
	if calendarPeopleIncludesAll(people) {
		return "@all"
	}
	if !hasPeople {
		return ""
	}
	return strings.Join(calendarMattermostMentionsForPeople(people, mattermostUsers), " ")
}

func calendarPeopleIncludesAll(people []string) bool {
	for _, person := range people {
		normalizedPerson := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(person, "@")))
		if normalizedPerson == "all" || normalizedPerson == "전체" {
			return true
		}
	}
	return false
}

func calendarMattermostMentionsForPeople(people []string, mattermostUsers []mattermostUserRecord) []string {
	mentions := []string{}
	seenMentions := map[string]bool{}
	for _, person := range people {
		mention := calendarMattermostMentionForPerson(person, mattermostUsers)
		if mention == "" || seenMentions[strings.ToLower(mention)] {
			continue
		}
		seenMentions[strings.ToLower(mention)] = true
		mentions = append(mentions, mention)
	}
	return mentions
}

func calendarMattermostMentionForPerson(person string, mattermostUsers []mattermostUserRecord) string {
	if user, found := calendarMattermostUserForPerson(person, mattermostUsers); found && strings.TrimSpace(user.Username) != "" {
		return "@" + strings.TrimSpace(user.Username)
	}
	mentionName := strings.TrimSpace(strings.TrimPrefix(person, "@"))
	if mentionName == "" || strings.ContainsAny(mentionName, " \t\r\n") {
		return ""
	}
	return "@" + mentionName
}

func calendarMattermostLogProps(event calendarEvent) map[string]any {
	return map[string]any{
		"internkim_calendar_event":    true,
		"internkim_calendar_event_id": event.ID,
	}
}

func calendarDisplayLocation(eventTimeZone string, fallbackLocation *time.Location) *time.Location {
	trimmedTimeZone := strings.TrimSpace(eventTimeZone)
	if trimmedTimeZone != "" && !strings.EqualFold(trimmedTimeZone, "UTC") {
		if location, errorValue := time.LoadLocation(trimmedTimeZone); errorValue == nil {
			return location
		}
	}
	if fallbackLocation != nil {
		return fallbackLocation
	}
	return time.UTC
}

func calendarMattermostEventDateText(event calendarEvent, fallbackLocation *time.Location) string {
	startTime, startError := time.Parse(time.RFC3339, strings.TrimSpace(event.StartISO))
	endTime, endError := time.Parse(time.RFC3339, strings.TrimSpace(event.EndISO))
	if startError != nil || endError != nil {
		return strings.TrimSpace(event.StartISO + " - " + event.EndISO)
	}
	location := calendarDisplayLocation(event.TimeZone, fallbackLocation)
	startTime = startTime.In(location)
	endTime = endTime.In(location)
	if event.IsAllDay {
		return startTime.Format("2006-01-02")
	}
	return startTime.Format("2006-01-02 15:04") + " - " + endTime.Format("15:04")
}

func calendarMattermostNoteText(description string) string {
	lines := strings.Split(strings.TrimSpace(description), "\n")
	if len(lines) == 0 {
		return ""
	}
	_, hasPeopleLine := calendarPeopleFromDescription(description)
	if hasPeopleLine && len(lines) > 1 {
		return strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}
	if hasPeopleLine {
		return ""
	}
	return strings.TrimSpace(description)
}

func (service *Service) mattermostCalendarLink(startISO string) string {
	label := service.workspaceText().CalendarOpen
	return "[" + label + "](" + service.mattermostCalendarURL(startISO) + ")"
}

func (service *Service) mattermostCalendarURL(startISO string) string {
	path := "/calendar/"
	if startTime, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(startISO)); errorValue == nil {
		path += "?date=" + url.QueryEscape(startTime.Format("2006-01-02"))
	}
	baseURL := strings.TrimRight(strings.TrimSpace(service.taskLinkBaseURL()), "/")
	if baseURL == "" {
		return path
	}
	return baseURL + path
}

// The record keys an event through the task it was paired with, not by the
// identifier this device made for it, and a link carrying the wrong one opens
// nothing. An event the record has not taken has nothing to send, and the link
// opens the day it is on.
func (service *Service) linkedCalendarEventID(eventID string) string {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" || !service.linksGoToTheRecord() {
		return eventID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	task, found, errorValue := service.readTaskByCalendarEventID(ctx, eventID)
	if errorValue != nil {
		log.Printf("the task paired with event %s is unreadable: %v", eventID, errorValue)
		return ""
	}
	if !found {
		return ""
	}
	return service.recordTaskIDCarrying(task.ID)
}

func (service *Service) mattermostCalendarEventURL(event calendarEvent) string {
	query := url.Values{}
	if startTime, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(event.StartISO)); errorValue == nil {
		query.Set("date", startTime.Format("2006-01-02"))
	}
	if linkedID := service.linkedCalendarEventID(event.ID); linkedID != "" {
		query.Set("event", linkedID)
	}
	path := "/calendar/"
	if encodedQuery := query.Encode(); encodedQuery != "" {
		path += "?" + encodedQuery
	}
	baseURL := strings.TrimRight(strings.TrimSpace(service.taskLinkBaseURL()), "/")
	if baseURL == "" {
		return path
	}
	return baseURL + path
}

func (service *Service) startCalendarNotificationWorker(ctx context.Context) {
	go func() {
		service.processDueCalendarNotifications(ctx, time.Now().UTC())
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				service.processDueCalendarNotifications(ctx, now.UTC())
			}
		}
	}()
}

func (service *Service) upsertCalendarNotifications(ctx context.Context, event calendarEvent) error {
	notifyAt, shouldNotify := calendarNotificationTime(event, time.Now().UTC())
	if !shouldNotify {
		return service.cancelCalendarNotifications(ctx, event.ID)
	}
	targets, errorValue := service.calendarNotificationTargets(ctx, event)
	if errorValue != nil {
		return fmt.Errorf("resolve calendar notification targets: %w", errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	targetKeys := map[string]bool{}
	for _, target := range targets {
		targetKeys[target.Key] = true
		if errorValue := upsertCalendarNotificationTarget(ctx, database, event.ID, target, notifyAt, updatedAt); errorValue != nil {
			return errorValue
		}
	}
	return service.cancelStaleCalendarNotifications(ctx, database, event.ID, targetKeys, updatedAt)
}

func upsertCalendarNotificationTarget(ctx context.Context, database *sql.DB, eventID string, target calendarNotificationTarget, notifyAt time.Time, updatedAt string) error {
	_, errorValue := database.ExecContext(ctx, `
INSERT INTO calendar_event_notifications (
	event_id, recipient_key, target_type, target_value, target_label, notify_at, status, sent_at, error, updated_at
) VALUES (?, ?, ?, ?, ?, ?, 'pending', '', '', ?)
ON CONFLICT(event_id, recipient_key) DO UPDATE SET
	target_type = excluded.target_type,
	target_value = excluded.target_value,
	target_label = excluded.target_label,
	notify_at = excluded.notify_at,
	status = 'pending',
	sent_at = '',
	error = '',
	updated_at = excluded.updated_at
WHERE calendar_event_notifications.status != 'sent'
	OR calendar_event_notifications.notify_at != excluded.notify_at`,
		eventID,
		target.Key,
		target.TargetType,
		firstNonEmpty(target.UserID, target.Key),
		target.Label,
		notifyAt.Format(time.RFC3339),
		updatedAt,
	)
	return errorValue
}

func calendarNotificationTime(event calendarEvent, now time.Time) (time.Time, bool) {
	startTime, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(event.StartISO))
	if errorValue != nil || !startTime.After(now) {
		return time.Time{}, false
	}
	notifyAt := startTime.Add(-time.Duration(normalizeCalendarReminderLeadHours(event.ReminderLeadHours)) * time.Hour)
	notifyAt = calendarNotificationDeliveryTime(notifyAt, event.TimeZone)
	if notifyAt.Before(now) {
		return now, true
	}
	return notifyAt, true
}

func calendarNotificationDeliveryTime(notifyAt time.Time, timeZone string) time.Time {
	location := calendarNotificationLocation(timeZone)
	localNotifyAt := notifyAt.In(location)
	if localNotifyAt.Hour() >= calendarNotificationQuietStartHour {
		return time.Date(localNotifyAt.Year(), localNotifyAt.Month(), localNotifyAt.Day(), calendarNotificationQuietStartHour, 0, 0, 0, location).UTC()
	}
	if localNotifyAt.Hour() < calendarNotificationQuietEndHour {
		previousDay := localNotifyAt.AddDate(0, 0, -1)
		return time.Date(previousDay.Year(), previousDay.Month(), previousDay.Day(), calendarNotificationQuietStartHour, 0, 0, 0, location).UTC()
	}
	return notifyAt
}

func calendarNotificationLocation(timeZone string) *time.Location {
	location, errorValue := time.LoadLocation(firstNonEmpty(strings.TrimSpace(timeZone), "UTC"))
	if errorValue != nil {
		return time.UTC
	}
	return location
}

func (service *Service) calendarNotificationTargets(ctx context.Context, event calendarEvent) ([]calendarNotificationTarget, error) {
	if _, hasPeople := calendarNotificationPeople(event); !hasPeople {
		return []calendarNotificationTarget{calendarAnnouncementsTarget()}, nil
	}
	users, errorValue := service.calendarMattermostUsers(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	targets, resolved := calendarNotificationTargetsForUsers(event, users)
	if !resolved {
		return []calendarNotificationTarget{calendarAnnouncementsTarget()}, nil
	}
	return targets, nil
}

func calendarNotificationTargetsForUsers(event calendarEvent, users []mattermostUserRecord) ([]calendarNotificationTarget, bool) {
	people, hasPeople := calendarNotificationPeople(event)
	if !hasPeople {
		return nil, false
	}
	targets := calendarTargetsForPeople(people, users)
	if len(targets) != len(people) {
		return nil, false
	}
	creatorTargets := calendarTargetsForPeople(calendarNotificationCreatorPeople(event), users)
	return appendCalendarNotificationTargets(targets, creatorTargets), true
}

func appendCalendarNotificationTargets(targets []calendarNotificationTarget, values []calendarNotificationTarget) []calendarNotificationTarget {
	seenKeys := map[string]bool{}
	for _, target := range targets {
		seenKeys[target.Key] = true
	}
	for _, value := range values {
		if value.Key == "" || seenKeys[value.Key] {
			continue
		}
		seenKeys[value.Key] = true
		targets = append(targets, value)
	}
	return targets
}

func calendarAnnouncementsTarget() calendarNotificationTarget {
	return calendarNotificationTarget{
		TargetType: "announcements",
		Key:        "announcements:" + calendarAnnouncementsChannelName,
		Label:      announcementsChannelDisplayName(workspaceLanguageKorean),
	}
}

func calendarPeopleFromDescription(description string) ([]string, bool) {
	lines := strings.Split(strings.TrimSpace(description), "\n")
	if len(lines) == 0 {
		return nil, false
	}
	firstLine := strings.TrimSpace(lines[0])
	if firstLine == "" {
		return nil, false
	}
	if !strings.Contains(firstLine, ",") && strings.ContainsAny(firstLine, " \t") {
		return nil, false
	}
	people := normalizeCalendarPeople(strings.Split(firstLine, ","))
	return people, len(people) > 0
}

func (service *Service) calendarMattermostUsers(ctx context.Context) ([]mattermostUserRecord, error) {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.activeMattermostUsers(ctx, token)
}

func calendarTargetsForPeople(people []string, users []mattermostUserRecord) []calendarNotificationTarget {
	targets := []calendarNotificationTarget{}
	seenUserIDs := map[string]bool{}
	for _, person := range people {
		user, found := calendarMattermostUserForPerson(person, users)
		if !found || seenUserIDs[user.ID] {
			continue
		}
		seenUserIDs[user.ID] = true
		targets = append(targets, calendarNotificationTarget{
			TargetType: "dm",
			Key:        "dm:" + user.ID,
			Label:      calendarMattermostUserDisplayName(user),
			UserID:     user.ID,
		})
	}
	return targets
}

func calendarMattermostUserForPerson(person string, users []mattermostUserRecord) (mattermostUserRecord, bool) {
	normalizedPerson := strings.ToLower(strings.TrimSpace(person))
	if normalizedPerson == "" {
		return mattermostUserRecord{}, false
	}
	for _, user := range users {
		for _, value := range calendarMattermostUserMatchValues(user) {
			if normalizedPerson == strings.ToLower(strings.TrimSpace(value)) {
				return user, true
			}
		}
	}
	return mattermostUserRecord{}, false
}

func calendarMattermostUserMatchValues(user mattermostUserRecord) []string {
	fullName := strings.TrimSpace(strings.Join([]string{user.FirstName, user.LastName}, " "))
	return uniqueNonEmpty([]string{
		user.ID,
		user.Email,
		user.Username,
		user.DisplayName,
		user.Nickname,
		user.FirstName,
		user.LastName,
		fullName,
	})
}

func calendarMattermostUserDisplayName(user mattermostUserRecord) string {
	return firstNonEmpty(user.Nickname, user.DisplayName, strings.TrimSpace(strings.Join([]string{user.FirstName, user.LastName}, " ")), user.Username, user.Email)
}

func (service *Service) cancelStaleCalendarNotifications(ctx context.Context, database *sql.DB, eventID string, targetKeys map[string]bool, updatedAt string) error {
	rows, errorValue := database.QueryContext(ctx, "SELECT recipient_key FROM calendar_event_notifications WHERE event_id = ? AND status = 'pending'", eventID)
	if errorValue != nil {
		return errorValue
	}
	staleRecipientKeys := []string{}
	for rows.Next() {
		var recipientKey string
		if errorValue := rows.Scan(&recipientKey); errorValue != nil {
			_ = rows.Close()
			return errorValue
		}
		if !targetKeys[recipientKey] {
			staleRecipientKeys = append(staleRecipientKeys, recipientKey)
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		_ = rows.Close()
		return errorValue
	}
	if errorValue := rows.Close(); errorValue != nil {
		return errorValue
	}
	for _, recipientKey := range staleRecipientKeys {
		if _, errorValue := database.ExecContext(ctx, "UPDATE calendar_event_notifications SET status = 'canceled', updated_at = ? WHERE event_id = ? AND recipient_key = ? AND status = 'pending'", updatedAt, eventID, recipientKey); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) cancelCalendarNotifications(ctx context.Context, eventID string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, "UPDATE calendar_event_notifications SET status = 'canceled', updated_at = ? WHERE event_id = ? AND status = 'pending'", time.Now().UTC().Format(time.RFC3339Nano), strings.TrimSpace(eventID))
	return errorValue
}

func (service *Service) processDueCalendarNotifications(ctx context.Context, now time.Time) {
	notifications, errorValue := service.readDueCalendarNotifications(ctx, now)
	if errorValue != nil {
		log.Printf("calendar notification read failed: %v", errorValue)
		return
	}
	for _, notification := range notifications {
		event, found, errorValue := service.readCalendarEventByID(ctx, notification.EventID)
		if errorValue != nil || !found {
			service.markCalendarNotificationError(ctx, notification, firstNonEmpty(errorString(errorValue), "event not found"))
			continue
		}
		if errorValue := service.sendCalendarNotification(ctx, notification, event); errorValue != nil {
			service.markCalendarNotificationError(ctx, notification, errorValue.Error())
			continue
		}
		service.markCalendarNotificationSent(ctx, notification)
	}
}

func (service *Service) readDueCalendarNotifications(ctx context.Context, now time.Time) ([]calendarNotification, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT event_id, recipient_key, target_type, target_value, target_label, notify_at, status
FROM calendar_event_notifications
WHERE status = 'pending' AND notify_at <= ?
ORDER BY notify_at`, now.UTC().Format(time.RFC3339))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	notifications := []calendarNotification{}
	for rows.Next() {
		var notification calendarNotification
		if errorValue := rows.Scan(&notification.EventID, &notification.RecipientKey, &notification.TargetType, &notification.TargetValue, &notification.TargetLabel, &notification.NotifyAt, &notification.Status); errorValue != nil {
			return nil, errorValue
		}
		notifications = append(notifications, notification)
	}
	return notifications, rows.Err()
}

func (service *Service) sendCalendarNotification(ctx context.Context, notification calendarNotification, event calendarEvent) error {
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	botToken, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		return errorValue
	}
	recipientUserID, errorValue := service.calendarReminderRecipientUserID(ctx, adminToken, notification, event)
	if errorValue != nil || recipientUserID == "" {
		return errorValue
	}
	channelID, errorValue := service.ensureMattermostBotDirectChannelID(ctx, adminToken, recipientUserID)
	if errorValue != nil {
		return errorValue
	}
	return service.postCalendarMattermostNotification(ctx, botToken, channelID, notification.TargetType, event)
}

func (service *Service) calendarReminderRecipientUserID(ctx context.Context, adminToken string, notification calendarNotification, event calendarEvent) (string, error) {
	if notification.TargetType == "dm" {
		return strings.TrimSpace(notification.TargetValue), nil
	}
	mattermostUsers, errorValue := service.activeMattermostUsers(ctx, adminToken)
	if errorValue != nil {
		return "", errorValue
	}
	author, found := calendarMattermostUserForPerson(calendarEventCausedByName(event), mattermostUsers)
	if !found {
		return "", nil
	}
	return author.ID, nil
}

func (service *Service) ensureMattermostBotDirectChannelID(ctx context.Context, token string, userID string) (string, error) {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return "", fmt.Errorf("Mattermost user ID is required")
	}
	botRecord, found, errorValue := service.findMattermostUserByUsername(ctx, token, service.Configuration.BotUsername)
	if errorValue != nil {
		return "", errorValue
	}
	if !found || botRecord.ID == "" || botRecord.DeleteAt != 0 || botRecord.ID == normalizedUserID {
		return "", fmt.Errorf("internkim bot user is not available")
	}
	body := []string{normalizedUserID, botRecord.ID}
	var channelRecord mattermostChannelRecord
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/direct", token, body, &channelRecord); errorValue != nil && !isMattermostBadRequest(errorValue) {
		return "", errorValue
	}
	if errorValue := service.showMattermostDirectChannel(ctx, token, normalizedUserID, botRecord.ID); errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(channelRecord.ID) != "" {
		return channelRecord.ID, nil
	}
	return "", fmt.Errorf("Mattermost direct channel was not created")
}

func (service *Service) postCalendarMattermostNotification(ctx context.Context, token string, channelID string, targetType string, event calendarEvent) error {
	body := map[string]any{
		"channel_id": channelID,
		"message":    service.calendarMattermostNotificationMessage(event, targetType),
		"props":      map[string]any{"internkim_calendar_notification": true, "calendar_event_id": event.ID},
	}
	return service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", token, body, nil)
}

func (service *Service) calendarMattermostNotificationMessage(event calendarEvent, targetType string) string {
	workspaceLocation, _ := service.workspaceTimeLocation()
	lines := []string{
		"Calendar reminder",
		"**" + mattermostMarkdownLink(event.Title, service.mattermostCalendarEventURL(event)) + "**",
		"Time: " + calendarNotificationTimeText(event, workspaceLocation),
	}
	if strings.TrimSpace(event.Location) != "" {
		lines = append(lines, "Location: "+strings.TrimSpace(event.Location))
	}
	if note := calendarNotificationNoteText(event.Description, targetType); note != "" {
		lines = append(lines, "Note: "+note)
	}
	return strings.Join(lines, "\n")
}

func calendarNotificationTimeText(event calendarEvent, fallbackLocation *time.Location) string {
	startTime, startError := time.Parse(time.RFC3339, event.StartISO)
	endTime, endError := time.Parse(time.RFC3339, event.EndISO)
	if startError != nil || endError != nil {
		return strings.TrimSpace(event.StartISO + " - " + event.EndISO)
	}
	location := calendarDisplayLocation(event.TimeZone, fallbackLocation)
	if event.IsAllDay {
		return startTime.In(location).Format("2006-01-02") + " all day"
	}
	return startTime.In(location).Format("2006-01-02 15:04") + " - " + endTime.In(location).Format("15:04 MST")
}

func calendarNotificationNoteText(description string, targetType string) string {
	lines := strings.Split(strings.TrimSpace(description), "\n")
	if len(lines) == 0 {
		return ""
	}
	if targetType == "dm" && len(lines) > 1 {
		return strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}
	if targetType == "dm" {
		return ""
	}
	return strings.TrimSpace(description)
}

func (service *Service) markCalendarNotificationSent(ctx context.Context, notification calendarNotification) {
	service.updateCalendarNotificationStatus(ctx, notification, "sent", time.Now().UTC().Format(time.RFC3339Nano), "")
}

func (service *Service) markCalendarNotificationError(ctx context.Context, notification calendarNotification, message string) {
	service.updateCalendarNotificationStatus(ctx, notification, "pending", "", message)
}

func (service *Service) updateCalendarNotificationStatus(ctx context.Context, notification calendarNotification, status string, sentAt string, message string) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		log.Printf("calendar notification status open failed: %v", errorValue)
		return
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, "UPDATE calendar_event_notifications SET status = ?, sent_at = ?, error = ?, updated_at = ? WHERE event_id = ? AND recipient_key = ?", status, sentAt, strings.TrimSpace(message), time.Now().UTC().Format(time.RFC3339Nano), notification.EventID, notification.RecipientKey)
	if errorValue != nil {
		log.Printf("calendar notification status update failed: %v", errorValue)
	}
}

func errorString(errorValue error) string {
	if errorValue == nil {
		return ""
	}
	return errorValue.Error()
}

func (service *Service) calendarNoticeFacts(event calendarEvent, change string) []string {
	workspaceLocation, _ := service.workspaceTimeLocation()
	return []string{
		"변경: " + change,
		"일정: " + strings.TrimSpace(event.Title),
		"일시: " + calendarMattermostEventDateText(event, workspaceLocation),
		"장소: " + strings.TrimSpace(event.Location),
		"참석자: " + strings.Join(calendarNoticeParticipantNames(event), ", "),
	}
}

func calendarNoticeParticipantNames(event calendarEvent) []string {
	names := append([]string{}, event.People...)
	for _, participant := range event.Participants {
		names = append(names, participant.Name)
	}
	return nonEmptyLines(names)
}
