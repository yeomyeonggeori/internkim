package admind

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

type changeRecipient struct {
	MattermostUserID string
	Name             string
}

func (service *Service) sendChangeDirectMessages(ctx context.Context, adminToken string, botToken string, recipients []changeRecipient, message string, props map[string]any, alreadyDelivered []string) ([]string, error) {
	delivered := append([]string{}, alreadyDelivered...)
	if strings.TrimSpace(message) == "" {
		return delivered, nil
	}
	var firstError error
	for _, recipient := range recipients {
		if containsString(delivered, recipient.MattermostUserID) {
			continue
		}
		errorValue := service.sendChangeDirectMessage(ctx, adminToken, botToken, recipient, message, props)
		if errorValue != nil {
			firstError = firstNonNilError(firstError, errorValue)
			continue
		}
		delivered = append(delivered, recipient.MattermostUserID)
	}
	return delivered, firstError
}

func (service *Service) sendChangeDirectMessage(ctx context.Context, adminToken string, botToken string, recipient changeRecipient, message string, props map[string]any) error {
	channelID, errorValue := service.ensureMattermostBotDirectChannelID(ctx, adminToken, recipient.MattermostUserID)
	if errorValue != nil {
		return errorValue
	}
	body := map[string]any{"channel_id": channelID, "message": message, "props": props}
	return service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", botToken, body, nil)
}

func changeNoticeKey(subjectID string, message string) string {
	digest := sha256.Sum256([]byte(message))
	return strings.TrimSpace(subjectID) + ":" + hex.EncodeToString(digest[:8])
}

func decodeDeliveredRecipients(storedValue string, noticeKey string) []string {
	var stored struct {
		NoticeKey  string   `json:"noticeKey"`
		Recipients []string `json:"recipients"`
	}
	if json.Unmarshal([]byte(storedValue), &stored) != nil || stored.NoticeKey != noticeKey {
		return nil
	}
	return stored.Recipients
}

func encodeDeliveredRecipients(noticeKey string, recipients []string) string {
	document, errorValue := json.Marshal(map[string]any{"noticeKey": noticeKey, "recipients": recipients})
	if errorValue != nil {
		return ""
	}
	return string(document)
}

func firstNonNilError(existing error, candidate error) error {
	if existing != nil {
		return existing
	}
	return candidate
}

func changeRecipientsForPeople(people []string, causedByName string, mattermostUsers []mattermostUserRecord) []changeRecipient {
	recipients := []changeRecipient{}
	seenUserID := map[string]bool{}
	for _, person := range append(append([]string{}, people...), causedByName) {
		user, found := calendarMattermostUserForPerson(person, mattermostUsers)
		if !found || strings.TrimSpace(user.ID) == "" || user.IsBot || user.DeleteAt != 0 {
			continue
		}
		if seenUserID[user.ID] {
			continue
		}
		seenUserID[user.ID] = true
		recipients = append(recipients, changeRecipient{MattermostUserID: user.ID, Name: strings.TrimSpace(person)})
	}
	return recipients
}

func flowTaskChangeRecipients(task flowTask, causedByName string, mattermostUsers []mattermostUserRecord) []changeRecipient {
	people := append([]string{task.OwnerName}, task.ParticipantNames...)
	return changeRecipientsForPeople(people, causedByName, mattermostUsers)
}

func calendarEventChangeRecipients(event calendarEvent, mattermostUsers []mattermostUserRecord) []changeRecipient {
	people := append([]string{}, event.People...)
	for _, participant := range event.Participants {
		people = append(people, participant.Name)
	}
	return changeRecipientsForPeople(people, calendarEventCausedByName(event), mattermostUsers)
}

func calendarEventCausedByName(event calendarEvent) string {
	if updatedByName := strings.TrimSpace(event.UpdatedByName); updatedByName != "" {
		return updatedByName
	}
	return strings.TrimSpace(event.CreatedByName)
}

func (service *Service) deliveredFlowRecipients(ctx context.Context, taskID string, noticeKey string) []string {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil
	}
	defer database.Close()
	var storedValue string
	if errorValue := database.QueryRowContext(ctx, "SELECT delivered_recipients FROM flow_channel_outbox WHERE task_id = ?", taskID).Scan(&storedValue); errorValue != nil {
		return nil
	}
	return decodeDeliveredRecipients(storedValue, noticeKey)
}

func (service *Service) recordDeliveredFlowRecipients(ctx context.Context, taskID string, noticeKey string, recipients []string) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return
	}
	defer database.Close()
	_, _ = database.ExecContext(ctx, "UPDATE flow_channel_outbox SET delivered_recipients = ? WHERE task_id = ?", encodeDeliveredRecipients(noticeKey, recipients), taskID)
}

func (service *Service) deliveredCalendarRecipients(ctx context.Context, eventID string, noticeKey string) []string {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil
	}
	defer database.Close()
	var storedValue string
	if errorValue := database.QueryRowContext(ctx, "SELECT delivered_recipients FROM calendar_channel_outbox WHERE event_id = ?", eventID).Scan(&storedValue); errorValue != nil {
		return nil
	}
	return decodeDeliveredRecipients(storedValue, noticeKey)
}

func (service *Service) recordDeliveredCalendarRecipients(ctx context.Context, eventID string, noticeKey string, recipients []string) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return
	}
	defer database.Close()
	_, _ = database.ExecContext(ctx, "UPDATE calendar_channel_outbox SET delivered_recipients = ? WHERE event_id = ?", encodeDeliveredRecipients(noticeKey, recipients), eventID)
}
