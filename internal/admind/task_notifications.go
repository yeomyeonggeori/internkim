package admind

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/fleetdomain"
)

func (service *Service) syncTaskMattermostNotification(ctx context.Context, task Task) Task {
	return service.applyTaskMattermostProjection(ctx, task)
}

func (service *Service) trySyncTaskMattermostNotification(ctx context.Context, task Task) (Task, error) {
	if strings.TrimSpace(service.Configuration.MattermostAdminPasswordPath) == "" {
		return task, nil
	}
	if !shouldNotifyTask(task) {
		return task, nil
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return task, errorValue
	}
	botToken, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		return task, errorValue
	}
	mattermostUsers, errorValue := service.activeMattermostUsers(ctx, adminToken)
	if errorValue != nil {
		return task, errorValue
	}
	recipients := taskChangeRecipients(task, "", mattermostUsers)
	message := service.changeNoticeWording(ctx, taskNoticeFacts(task), service.taskMattermostNotificationMessage(task, mattermostUsers))
	noticeKey := changeNoticeKey(task.ID, message)
	delivered, errorValue := service.sendChangeDirectMessages(ctx, adminToken, botToken, recipients, message, taskMattermostNotificationProps(task), service.deliveredTaskRecipients(ctx, task.ID, noticeKey))
	service.recordDeliveredTaskRecipients(ctx, task.ID, noticeKey, delivered)
	return task, errorValue
}

func shouldNotifyTask(task Task) bool {
	switch {
	case isTaskRequestedStatus(task.Status), isTaskRejectedStatus(task.Status), isTaskStoppedStatus(task.Status), isTaskCompletedStatus(task.Status):
		return true
	default:
		return false
	}
}

func (service *Service) ensureMattermostBotCanPost(ctx context.Context, adminToken string, channelID string, userID string) error {
	body := map[string]string{"user_id": userID}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/"+url.PathEscape(channelID)+"/members", adminToken, body, nil); errorValue != nil && !isMattermostBadRequest(errorValue) {
		return errorValue
	}
	schemeRoles := map[string]bool{"scheme_admin": true, "scheme_user": true}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/channels/"+url.PathEscape(channelID)+"/members/"+url.PathEscape(userID)+"/schemeRoles", adminToken, schemeRoles, nil)
}

func (service *Service) taskMattermostNotificationMessage(task Task, mattermostUsers []mattermostUserRecord) string {
	return mattermostMarkdownTable(
		[]string{"상태", "업무", "유형", "크기", "참여자"},
		[][]string{{
			task.Status,
			mattermostMarkdownLink(task.Content, service.mattermostTaskURL(task)),
			task.Type,
			task.Size,
			strings.Join(taskMattermostParticipantMentions(task, mattermostUsers), " "),
		}},
	)
}

func taskMattermostParticipantMentions(task Task, mattermostUsers []mattermostUserRecord) []string {
	mentions := make([]string, 0, len(task.ParticipantNames))
	for _, participantName := range task.ParticipantNames {
		if mention := calendarMattermostMentionForPerson(participantName, mattermostUsers); mention != "" {
			mentions = append(mentions, mention)
		}
	}
	return mentions
}

func taskMattermostNotificationProps(task Task) map[string]any {
	return map[string]any{
		"internkim_flow_task":      true,
		"internkim_flow_task_id":   task.ID,
		"internkim_flow_week_code": task.WeekCode,
	}
}

func (service *Service) mattermostTaskLink(weekCode string) string {
	label := service.workspaceText().TaskOpen
	return "[" + label + "](" + service.mattermostTaskPageURL(weekCode) + ")"
}

func (service *Service) mattermostTaskPageURL(weekCode string) string {
	path := "/flow/"
	weekCode = strings.TrimSpace(weekCode)
	if weekCode != "" {
		path += "?week=" + url.QueryEscape(weekCode)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(service.taskLinkBaseURL()), "/")
	if baseURL == "" {
		return path
	}
	return baseURL + path
}

func (service *Service) linksGoToTheRecord() bool {
	return strings.TrimSpace(service.Configuration.CentralPlaneAppURL) != ""
}

// An identifier belongs with the address it is sent to: the record keys a task
// by its own, this device by the one it made, and a link carrying the other
// one's opens nothing. A task the record has not taken yet has nothing to send,
// and the link opens the week it is in.
func (service *Service) linkedTaskID(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || !service.linksGoToTheRecord() {
		return taskID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	recordID, errorValue := service.readTaskCentralIdentityByTaskID(ctx, taskID)
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(recordID)
}

func (service *Service) mattermostTaskURL(task Task) string {
	query := url.Values{}
	if weekCode := strings.TrimSpace(task.WeekCode); weekCode != "" {
		query.Set("week", weekCode)
	}
	if linkedID := service.linkedTaskID(task.ID); linkedID != "" {
		query.Set("task", linkedID)
	}
	path := "/flow/"
	if encodedQuery := query.Encode(); encodedQuery != "" {
		path += "?" + encodedQuery
	}
	baseURL := strings.TrimRight(strings.TrimSpace(service.taskLinkBaseURL()), "/")
	if baseURL == "" {
		return path
	}
	return baseURL + path
}

// Everyone signs in at the company's own address and the record lives behind it,
// so that is where a link points. A device address is what is left for a company
// that has not moved.
func (service *Service) taskLinkBaseURL() string {
	if appURL := strings.TrimSpace(service.Configuration.CentralPlaneAppURL); appURL != "" {
		return appURL
	}
	if taskPublicURL := strings.TrimSpace(service.Configuration.TaskPublicURL); taskPublicURL != "" {
		return taskPublicURL
	}
	if written := strings.TrimSpace(readTrimmedFile(service.Configuration.TaskPublicURLPath)); written != "" {
		return written
	}
	return service.mattermostTaskBaseURL()
}

func (service *Service) mattermostTaskBaseURL() string {
	if deviceURL := strings.TrimSpace(readTrimmedFile(service.Configuration.DeviceURLPath)); deviceURL != "" {
		return deviceURL
	}
	if fleetID := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)); fleetID != "" {
		return fleetdomain.Subdomain(strings.ToLower(fleetID), service.fleetZone())
	}
	return ""
}

func taskNoticeFacts(task Task) []string {
	return []string{
		"업무: " + strings.TrimSpace(task.Content),
		"상태: " + strings.TrimSpace(task.Status),
		"담당자: " + strings.TrimSpace(task.OwnerName),
		"참여자: " + strings.Join(task.ParticipantNames, ", "),
		"기간: " + strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(task.StartDate+" ~ "+task.EndDate), "~")),
	}
}
