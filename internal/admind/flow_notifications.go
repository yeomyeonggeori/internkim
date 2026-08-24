package admind

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/fleetdomain"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

func (service *Service) syncFlowMattermostNotification(ctx context.Context, task flowTask) flowTask {
	return service.applyFlowMattermostProjection(ctx, task)
}

func (service *Service) trySyncFlowMattermostNotification(ctx context.Context, task flowTask) (flowTask, error) {
	if strings.TrimSpace(service.Configuration.MattermostAdminPasswordPath) == "" {
		return task, nil
	}
	if !shouldNotifyFlowTask(task) {
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
	recipients := flowTaskChangeRecipients(task, "", mattermostUsers)
	message := service.changeNoticeWording(ctx, flowNoticeFacts(task), service.flowMattermostNotificationMessage(task, mattermostUsers))
	noticeKey := changeNoticeKey(task.ID, message)
	delivered, errorValue := service.sendChangeDirectMessages(ctx, adminToken, botToken, recipients, message, flowMattermostNotificationProps(task), service.deliveredFlowRecipients(ctx, task.ID, noticeKey))
	service.recordDeliveredFlowRecipients(ctx, task.ID, noticeKey, delivered)
	return task, errorValue
}

func shouldNotifyFlowTask(task flowTask) bool {
	switch {
	case isFlowRequestedStatus(task.Status), isFlowRejectedStatus(task.Status), isFlowStoppedStatus(task.Status), isFlowCompletedStatus(task.Status):
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

func (service *Service) flowMattermostNotificationMessage(task flowTask, mattermostUsers []mattermostUserRecord) string {
	return mattermostMarkdownTable(
		[]string{"상태", "업무", "유형", "크기", "참여자"},
		[][]string{{
			task.Status,
			mattermostMarkdownLink(task.Content, service.mattermostFlowTaskURL(task)),
			task.Type,
			task.Size,
			strings.Join(flowMattermostParticipantMentions(task, mattermostUsers), " "),
		}},
	)
}

func flowMattermostParticipantMentions(task flowTask, mattermostUsers []mattermostUserRecord) []string {
	mentions := make([]string, 0, len(task.ParticipantNames))
	for _, participantName := range task.ParticipantNames {
		if mention := calendarMattermostMentionForPerson(participantName, mattermostUsers); mention != "" {
			mentions = append(mentions, mention)
		}
	}
	return mentions
}

func flowMattermostNotificationProps(task flowTask) map[string]any {
	return map[string]any{
		"internkim_flow_task":      true,
		"internkim_flow_task_id":   task.ID,
		"internkim_flow_week_code": task.WeekCode,
	}
}

func (service *Service) mattermostFlowLink(weekCode string) string {
	label := mattermostdefaults.PublicChannelLinkLabel(mattermostFlowChannelName, service.workspaceLanguage())
	return "[" + label + "](" + service.mattermostFlowURL(weekCode) + ")"
}

func (service *Service) mattermostFlowURL(weekCode string) string {
	path := "/flow/"
	weekCode = strings.TrimSpace(weekCode)
	if weekCode != "" {
		path += "?week=" + url.QueryEscape(weekCode)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(service.flowLinkBaseURL()), "/")
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
	recordID, errorValue := service.readFlowCentralIdentityByTaskID(ctx, taskID)
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(recordID)
}

func (service *Service) mattermostFlowTaskURL(task flowTask) string {
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
	baseURL := strings.TrimRight(strings.TrimSpace(service.flowLinkBaseURL()), "/")
	if baseURL == "" {
		return path
	}
	return baseURL + path
}

// Everyone signs in at the company's own address and the record lives behind it,
// so that is where a link points. A device address is what is left for a company
// that has not moved.
func (service *Service) flowLinkBaseURL() string {
	if appURL := strings.TrimSpace(service.Configuration.CentralPlaneAppURL); appURL != "" {
		return appURL
	}
	if flowPublicURL := strings.TrimSpace(service.Configuration.FlowPublicURL); flowPublicURL != "" {
		return flowPublicURL
	}
	if written := strings.TrimSpace(readTrimmedFile(service.Configuration.FlowPublicURLPath)); written != "" {
		return written
	}
	return service.mattermostFlowBaseURL()
}

func (service *Service) mattermostFlowBaseURL() string {
	if deviceURL := strings.TrimSpace(readTrimmedFile(service.Configuration.DeviceURLPath)); deviceURL != "" {
		return deviceURL
	}
	if fleetID := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)); fleetID != "" {
		return fleetdomain.Subdomain(strings.ToLower(fleetID), service.fleetZone())
	}
	return ""
}

func flowNoticeFacts(task flowTask) []string {
	return []string{
		"업무: " + strings.TrimSpace(task.Content),
		"상태: " + strings.TrimSpace(task.Status),
		"담당자: " + strings.TrimSpace(task.OwnerName),
		"참여자: " + strings.Join(task.ParticipantNames, ", "),
		"기간: " + strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(task.StartDate+" ~ "+task.EndDate), "~")),
	}
}
