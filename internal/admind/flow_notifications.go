package admind

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

func (service *Service) syncFlowMattermostNotification(ctx context.Context, task flowTask) flowTask {
	return service.applyFlowMattermostProjection(ctx, task)
}

func (service *Service) trySyncFlowMattermostNotification(ctx context.Context, task flowTask) (flowTask, error) {
	if strings.TrimSpace(service.Configuration.MattermostAdminPasswordPath) == "" {
		return task, nil
	}
	if !shouldNotifyFlowTask(task) && strings.TrimSpace(task.MattermostPostID) == "" {
		return task, nil
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return task, errorValue
	}
	if !shouldNotifyFlowTask(task) {
		return service.deleteFlowMattermostNotification(ctx, adminToken, task)
	}
	botToken, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		return task, errorValue
	}
	botUserID, errorValue := service.mattermostTokenUserID(ctx, botToken)
	if errorValue != nil {
		return task, errorValue
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, adminToken)
	if errorValue != nil {
		return task, errorValue
	}
	channelID, errorValue := service.ensureMattermostFlowChannel(ctx, adminToken, teamRecord.ID)
	if errorValue != nil {
		return task, errorValue
	}
	if errorValue := service.ensureMattermostBotCanPost(ctx, adminToken, channelID, botUserID); errorValue != nil {
		return task, errorValue
	}
	return service.upsertFlowMattermostNotification(ctx, adminToken, botToken, botUserID, channelID, task)
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

func (service *Service) upsertFlowMattermostNotification(ctx context.Context, adminToken string, botToken string, botUserID string, channelID string, task flowTask) (flowTask, error) {
	if strings.TrimSpace(task.MattermostPostID) == "" {
		return service.createFlowMattermostNotification(ctx, botToken, channelID, task)
	}
	postRecord, found, errorValue := service.mattermostPostByID(ctx, adminToken, task.MattermostPostID)
	if errorValue != nil {
		return task, errorValue
	}
	if !found || strings.TrimSpace(postRecord.UserID) != botUserID {
		task, errorValue = service.deleteFlowMattermostNotification(ctx, adminToken, task)
		if errorValue != nil {
			return task, errorValue
		}
		return service.createFlowMattermostNotification(ctx, botToken, channelID, task)
	}
	body := map[string]any{
		"message": service.flowMattermostNotificationMessage(task),
		"props":   flowMattermostNotificationProps(task),
	}
	path := "/api/v4/posts/" + url.PathEscape(task.MattermostPostID) + "/patch"
	if errorValue := service.mattermostRequest(ctx, http.MethodPut, path, botToken, body, nil); errorValue != nil {
		if !isMattermostNotFound(errorValue) && !isMattermostForbidden(errorValue) {
			return task, errorValue
		}
		task, errorValue = service.deleteFlowMattermostNotification(ctx, adminToken, task)
		if errorValue != nil {
			return task, errorValue
		}
		return service.createFlowMattermostNotification(ctx, botToken, channelID, task)
	}
	return task, nil
}

func (service *Service) mattermostPostByID(ctx context.Context, token string, postID string) (mattermostPostRecord, bool, error) {
	var postRecord mattermostPostRecord
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/posts/"+url.PathEscape(postID), token, nil, &postRecord)
	if errorValue == nil && postRecord.ID != "" {
		return postRecord, true, nil
	}
	if errorValue != nil && !isMattermostNotFound(errorValue) {
		return mattermostPostRecord{}, false, errorValue
	}
	return mattermostPostRecord{}, false, nil
}

func (service *Service) createFlowMattermostNotification(ctx context.Context, token string, channelID string, task flowTask) (flowTask, error) {
	body := map[string]any{
		"channel_id": channelID,
		"message":    service.flowMattermostNotificationMessage(task),
		"props":      flowMattermostNotificationProps(task),
	}
	var response struct {
		ID string `json:"id"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", token, body, &response); errorValue != nil {
		return task, errorValue
	}
	task.MattermostPostID = strings.TrimSpace(response.ID)
	if task.MattermostPostID == "" {
		return task, nil
	}
	return task, service.updateFlowTaskMattermostPostID(ctx, task.ID, task.MattermostPostID)
}

func (service *Service) deleteFlowMattermostNotification(ctx context.Context, token string, task flowTask) (flowTask, error) {
	if strings.TrimSpace(task.MattermostPostID) == "" {
		return task, nil
	}
	errorValue := service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/posts/"+url.PathEscape(task.MattermostPostID), token, nil, nil)
	if errorValue != nil {
		if !isMattermostNotFound(errorValue) {
			return task, errorValue
		}
	}
	task.MattermostPostID = ""
	return task, service.updateFlowTaskMattermostPostID(ctx, task.ID, "")
}

func (service *Service) flowMattermostNotificationMessage(task flowTask) string {
	return mattermostMarkdownTable(
		[]string{"상태", "담당", "업무", "유형", "크기", "참여자"},
		[][]string{{
			task.Status,
			task.OwnerName,
			mattermostMarkdownLink(task.Content, service.mattermostFlowTaskURL(task)),
			task.Type,
			task.Size,
			strings.Join(task.ParticipantNames, ", "),
		}},
	)
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

func (service *Service) mattermostFlowTaskURL(task flowTask) string {
	query := url.Values{}
	if weekCode := strings.TrimSpace(task.WeekCode); weekCode != "" {
		query.Set("week", weekCode)
	}
	if taskID := strings.TrimSpace(task.ID); taskID != "" {
		query.Set("task", taskID)
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

func (service *Service) flowLinkBaseURL() string {
	if flowPublicURL := strings.TrimSpace(service.Configuration.FlowPublicURL); flowPublicURL != "" {
		return flowPublicURL
	}
	return service.mattermostFlowBaseURL()
}

func (service *Service) mattermostFlowBaseURL() string {
	if deviceURL := strings.TrimSpace(readTrimmedFile(service.Configuration.DeviceURLPath)); deviceURL != "" {
		return deviceURL
	}
	if fleetID := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)); fleetID != "" {
		return "https://" + strings.ToLower(fleetID) + ".example.test"
	}
	return ""
}
