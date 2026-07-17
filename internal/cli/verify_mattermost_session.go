package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	mattermostAdminPasswordPath = "/root/.internkim/secrets/mm-admin-pass"
	mattermostBotTokenPath      = "/root/.internkim/secrets/mattermost-bot-token"
)

var mattermostScenarioURLPattern = regexp.MustCompile(`https?://[^\s<>()]+`)

type mattermostScenarioExecution struct {
	Result      mattermostScenarioResult `json:"result"`
	Username    string                   `json:"-"`
	Password    string                   `json:"-"`
	RootPostID  string                   `json:"-"`
	BotUsername string                   `json:"-"`
}

type mattermostScenarioStepHook func(context.Context, mattermostScenarioExecution, int) error

type mattermostScenarioSession struct {
	scenario          mattermostScenario
	mattermost        mattermostProbeAPI
	admin             mattermostScenarioAdminAPI
	adminToken        string
	userToken         string
	botUserID         string
	botUsername       string
	username          string
	password          string
	email             string
	userID            string
	channelID         string
	rootPostID        string
	seenBotPostIDs    map[string]bool
	result            mattermostScenarioResult
	startedAt         time.Time
	poll              func(context.Context) error
	shouldAutoConfirm bool
	cleanupMutex      sync.Mutex
	isCleanupFinished bool
}

type mattermostScenarioTaskSnapshot struct {
	UpdatedAt string
	EventIDs  map[string]bool
}

func startMattermostScenarioSession(contextValue context.Context, target verifyTarget, mattermostURL string, scenario mattermostScenario) (*mattermostScenarioSession, error) {
	remote := target.scenarioRemote
	if remote == nil && target.sshClient != nil {
		remote = mattermostScenarioSSHRemote{client: target.sshClient}
	}
	if remote == nil {
		return nil, errors.New("Mattermost scenario requires a Local Fleet SSH target")
	}
	mattermost, errorValue := newMattermostProbeClient(mattermostURL)
	if errorValue != nil {
		return nil, errorValue
	}
	admin := mattermostScenarioAdmin{remote: remote}
	session := newMattermostScenarioSession(scenario, mattermost, admin)
	if errorValue := session.setup(contextValue); errorValue != nil {
		cleanupContext, cancelCleanup := context.WithTimeout(context.Background(), 2*time.Minute)
		cleanupError := session.cleanup(cleanupContext)
		cancelCleanup()
		return nil, errors.Join(errorValue, cleanupError)
	}
	return session, nil
}

func newMattermostScenarioSession(scenario mattermostScenario, mattermost mattermostProbeAPI, admin mattermostScenarioAdminAPI) *mattermostScenarioSession {
	return &mattermostScenarioSession{
		scenario:       scenario,
		mattermost:     mattermost,
		admin:          admin,
		seenBotPostIDs: map[string]bool{},
		poll:           waitForMattermostScenarioPoll,
		result: mattermostScenarioResult{
			ScenarioName: scenario.Name,
			TurnCount:    len(scenario.Steps),
		},
	}
}

func (session *mattermostScenarioSession) setup(contextValue context.Context) error {
	adminPassword, errorValue := session.admin.readSecret(contextValue, mattermostAdminPasswordPath)
	if errorValue != nil {
		return errorValue
	}
	botToken, errorValue := session.admin.readSecret(contextValue, mattermostBotTokenPath)
	if errorValue != nil {
		return errorValue
	}
	session.adminToken, errorValue = session.mattermost.Login(contextValue, "admin", adminPassword)
	if errorValue != nil {
		return errorValue
	}
	botUser, errorValue := session.mattermost.CurrentUser(contextValue, botToken)
	if errorValue != nil {
		return errorValue
	}
	session.botUserID = botUser.ID
	session.botUsername = botUser.Username
	suffix := randomHexString(6)
	session.username = "probescenario" + suffix
	session.password = "ProbePass!" + randomHexString(12)
	session.email = session.username + "@internkim.test"
	user, errorValue := session.mattermost.CreateUser(contextValue, session.adminToken, mattermostProbeNewUser{
		Username:  session.username,
		Email:     session.email,
		Password:  session.password,
		FirstName: "Expensive",
		LastName:  "Probe",
	})
	if errorValue != nil {
		return errorValue
	}
	session.userID = user.ID
	session.result.UserID = user.ID
	session.userToken, errorValue = session.mattermost.Login(contextValue, session.username, session.password)
	if errorValue != nil {
		return errorValue
	}
	team, errorValue := session.mattermost.TeamByName(contextValue, session.adminToken, "internkim")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := session.mattermost.AddTeamMember(contextValue, session.adminToken, team.ID, user.ID); errorValue != nil {
		return errorValue
	}
	if errorValue := session.admin.invitePerson(contextValue, user.ID, session.email, session.username); errorValue != nil {
		return errorValue
	}
	channel, errorValue := session.mattermost.CreateDirectChannel(contextValue, session.userToken, user.ID, botUser.ID)
	if errorValue != nil {
		return errorValue
	}
	session.channelID = channel.ID
	session.result.ChannelID = channel.ID
	return nil
}

func (session *mattermostScenarioSession) run(contextValue context.Context, hook mattermostScenarioStepHook) error {
	session.startedAt = time.Now()
	defer func() {
		session.result.ScenarioWallDurationMS = time.Since(session.startedAt).Milliseconds()
	}()
	for stepIndex := range session.scenario.Steps {
		if errorValue := session.runStep(contextValue, stepIndex); errorValue != nil {
			return errorValue
		}
		step := session.scenario.Steps[stepIndex]
		if step.ApprovalAction != "" && !session.shouldAutoConfirm {
			return errors.New("Mattermost scenario approval action requires --auto-confirm")
		}
		if hook != nil {
			if errorValue := hook(contextValue, session.execution(), stepIndex); errorValue != nil {
				return errorValue
			}
		}
		if step.ApprovalAction != "" {
			if errorValue := session.finishApprovalStep(contextValue, stepIndex); errorValue != nil {
				return errorValue
			}
			if errorValue := validateMattermostScenarioStep(stepIndex, session.scenario, step, session.result.Steps[stepIndex], &session.result); errorValue != nil {
				return errorValue
			}
		}
	}
	return validateMattermostScenarioResult(session.scenario, &session.result)
}

func (session *mattermostScenarioSession) runStep(contextValue context.Context, stepIndex int) error {
	step := session.scenario.Steps[stepIndex]
	startedAt := time.Now()
	session.result.Steps = append(session.result.Steps, mattermostScenarioStepResult{Prompt: step.Prompt})
	stepResult := &session.result.Steps[len(session.result.Steps)-1]
	defer func() {
		stepResult.ProcessingMS = time.Since(startedAt).Milliseconds()
	}()
	snapshot, errorValue := session.snapshotTasks(contextValue)
	if errorValue != nil {
		return errorValue
	}
	userPost, errorValue := session.postStep(contextValue, step.Prompt)
	if errorValue != nil {
		return errorValue
	}
	session.result.Posts = append(session.result.Posts, convertMattermostScenarioPost(userPost))
	sourceReference := session.stepSourceReference(userPost.ID)
	detail, events, replyPostID, errorValue := session.waitForStepTask(contextValue, snapshot, sourceReference)
	stepResult.TaskRunID = detail.TaskRun.TaskRunID
	stepResult.TaskStatus = detail.TaskRun.Status
	stepResult.TaskEvents = events
	setMattermostScenarioStepMetrics(stepResult)
	stepResult.PublicURL = findMattermostScenarioEventPublicURL(events)
	if errorValue != nil {
		return errorValue
	}
	botPost, errorValue := session.waitForStepReply(contextValue, replyPostID)
	if errorValue != nil {
		return errorValue
	}
	stepResult.BotMessage = botPost.Message
	session.result.Posts = append(session.result.Posts, convertMattermostScenarioPost(botPost))
	attachments, errorValue := session.downloadAttachments(contextValue, botPost.FileIDs)
	stepResult.Attachments = attachments
	if errorValue != nil {
		return errorValue
	}
	workspaceFiles, errorValue := session.admin.workspaceFiles(contextValue, step)
	if errorValue != nil {
		return errorValue
	}
	stepResult.WorkspaceFiles = workspaceFiles
	if step.ApprovalAction != "" {
		return nil
	}
	return validateMattermostScenarioStep(stepIndex, session.scenario, step, *stepResult, &session.result)
}

func (session *mattermostScenarioSession) finishApprovalStep(contextValue context.Context, stepIndex int) error {
	stepResult := &session.result.Steps[stepIndex]
	detail, newEvents, replyPostID, errorValue := session.waitForApprovalCompletion(contextValue, stepResult.TaskRunID, stepResult.TaskEvents)
	stepResult.TaskStatus = detail.TaskRun.Status
	stepResult.TaskEvents = append(stepResult.TaskEvents, newEvents...)
	setMattermostScenarioStepMetrics(stepResult)
	stepResult.PublicURL = findMattermostScenarioEventPublicURL(stepResult.TaskEvents)
	if errorValue != nil {
		return errorValue
	}
	botPost, errorValue := session.waitForStepReply(contextValue, replyPostID)
	if errorValue != nil {
		return errorValue
	}
	stepResult.BotMessage = botPost.Message
	session.result.Posts = append(session.result.Posts, convertMattermostScenarioPost(botPost))
	stepResult.Attachments, errorValue = session.downloadAttachments(contextValue, botPost.FileIDs)
	if errorValue != nil {
		return errorValue
	}
	stepResult.WorkspaceFiles, errorValue = session.admin.workspaceFiles(contextValue, session.scenario.Steps[stepIndex])
	return errorValue
}

func (session *mattermostScenarioSession) waitForApprovalCompletion(contextValue context.Context, taskRunID string, previousEvents []mattermostScenarioTaskEvent) (mattermostScenarioTaskDetail, []mattermostScenarioTaskEvent, string, error) {
	previousEventIDs := mattermostScenarioEventIDSet(previousEvents)
	for {
		detail, errorValue := session.admin.taskDetail(contextValue, taskRunID)
		if errorValue != nil {
			return mattermostScenarioTaskDetail{}, nil, "", errorValue
		}
		newEvents := mattermostScenarioNewEvents(detail.TaskEvents, previousEventIDs)
		replyPostID := mattermostScenarioReplyPostID(newEvents, detail.TaskRun.Status, "")
		if detail.TaskRun.Status != "waiting_approval" && !isMattermostScenarioTaskInProgress(detail.TaskRun.Status) && replyPostID != "" {
			return detail, newEvents, replyPostID, nil
		}
		if errorValue := session.poll(contextValue); errorValue != nil {
			return detail, newEvents, replyPostID, errorValue
		}
	}
}

func setMattermostScenarioStepMetrics(result *mattermostScenarioStepResult) {
	result.LLMCallCount = 0
	result.AgentStepCount = 0
	result.ToolCallCount = 0
	for _, event := range result.TaskEvents {
		if event.Name == "llm.call" {
			result.LLMCallCount++
		}
		if event.Name == "agent.action" {
			result.AgentStepCount++
		}
		if _, isDirectToolEvent := mattermostScenarioDirectToolName(event.Name, "requested"); isDirectToolEvent {
			result.ToolCallCount++
		}
	}
}

func (session *mattermostScenarioSession) postStep(contextValue context.Context, prompt string) (mattermostProbePost, error) {
	message := mattermostProbeMessage{ChannelID: session.channelID, RootID: session.rootPostID, Message: prompt}
	post, errorValue := session.mattermost.PostMessage(contextValue, session.userToken, message)
	if errorValue != nil {
		return mattermostProbePost{}, errorValue
	}
	if session.rootPostID == "" {
		session.rootPostID = post.ID
		session.result.ConversationID = session.conversationID()
	}
	return post, nil
}

func (session *mattermostScenarioSession) snapshotTasks(contextValue context.Context) (map[string]mattermostScenarioTaskSnapshot, error) {
	tasks, errorValue := session.admin.listTasks(contextValue, session.conversationID())
	if errorValue != nil {
		return nil, errorValue
	}
	snapshot := make(map[string]mattermostScenarioTaskSnapshot, len(tasks))
	for _, taskSummary := range tasks {
		detail, detailError := session.admin.taskDetail(contextValue, taskSummary.TaskRunID)
		if detailError != nil {
			return nil, detailError
		}
		snapshot[taskSummary.TaskRunID] = mattermostScenarioTaskSnapshot{
			UpdatedAt: taskSummary.UpdatedAt,
			EventIDs:  mattermostScenarioEventIDSet(detail.TaskEvents),
		}
	}
	return snapshot, nil
}

func (session *mattermostScenarioSession) waitForStepTask(contextValue context.Context, snapshot map[string]mattermostScenarioTaskSnapshot, sourceReference string) (mattermostScenarioTaskDetail, []mattermostScenarioTaskEvent, string, error) {
	var latestDetail mattermostScenarioTaskDetail
	var latestEvents []mattermostScenarioTaskEvent
	var latestReplyPostID string
	for {
		tasks, errorValue := session.admin.listTasks(contextValue, session.conversationID())
		if errorValue != nil {
			return latestDetail, latestEvents, latestReplyPostID, errorValue
		}
		for taskIndex := len(tasks) - 1; taskIndex >= 0; taskIndex-- {
			taskSummary := tasks[taskIndex]
			detail, detailError := session.admin.taskDetail(contextValue, taskSummary.TaskRunID)
			if detailError != nil {
				return latestDetail, latestEvents, latestReplyPostID, detailError
			}
			if !mattermostScenarioHasSourceReference(detail.TaskEvents, sourceReference) {
				continue
			}
			newEvents := mattermostScenarioNewEvents(detail.TaskEvents, snapshot[taskSummary.TaskRunID].EventIDs)
			previous, didExist := snapshot[taskSummary.TaskRunID]
			if !didExist || previous.UpdatedAt != taskSummary.UpdatedAt || len(newEvents) > 0 {
				latestDetail = detail
				latestEvents = newEvents
				latestReplyPostID = mattermostScenarioReplyPostID(newEvents, detail.TaskRun.Status, sourceReference)
			}
			if didExist && previous.UpdatedAt == taskSummary.UpdatedAt && len(newEvents) == 0 {
				continue
			}
			if len(newEvents) == 0 || isMattermostScenarioTaskInProgress(detail.TaskRun.Status) || latestReplyPostID == "" {
				continue
			}
			return detail, newEvents, latestReplyPostID, nil
		}
		if errorValue := session.poll(contextValue); errorValue != nil {
			return latestDetail, latestEvents, latestReplyPostID, errorValue
		}
	}
}

func (session *mattermostScenarioSession) conversationID() string {
	if session.rootPostID == "" {
		return session.channelID
	}
	return "thread:" + session.channelID + ":" + session.rootPostID
}

func (session *mattermostScenarioSession) stepSourceReference(postID string) string {
	return "mattermost:" + session.conversationID() + ":" + postID
}

func (session *mattermostScenarioSession) waitForStepReply(contextValue context.Context, replyPostID string) (mattermostProbePost, error) {
	for {
		posts, errorValue := session.mattermost.ListChannelPosts(contextValue, session.adminToken, session.channelID)
		if errorValue != nil {
			return mattermostProbePost{}, errorValue
		}
		for _, post := range posts {
			if post.ID != replyPostID || post.UserID != session.botUserID || post.RootID != session.rootPostID || session.seenBotPostIDs[post.ID] {
				continue
			}
			session.seenBotPostIDs[post.ID] = true
			return post, nil
		}
		if errorValue := session.poll(contextValue); errorValue != nil {
			return mattermostProbePost{}, errorValue
		}
	}
}

func mattermostScenarioHasSourceReference(events []mattermostScenarioTaskEvent, sourceReference string) bool {
	for _, event := range events {
		if event.Name != "agent.task_source" && event.Name != "agent.task_launched" {
			continue
		}
		var body struct {
			SourceReference string `json:"sourceReference"`
		}
		if json.Unmarshal([]byte(event.Body), &body) == nil && body.SourceReference == sourceReference {
			return true
		}
	}
	return false
}

func mattermostScenarioReplyPostID(events []mattermostScenarioTaskEvent, taskStatus string, sourceReference string) string {
	expectedReplyKind := "user_notice"
	if taskStatus == "completed" {
		expectedReplyKind = "success"
	}
	messageID := sourceReference[strings.LastIndex(sourceReference, ":")+1:]
	for eventIndex := len(events) - 1; eventIndex >= 0; eventIndex-- {
		event := events[eventIndex]
		if event.Name != "connector.reply.sent" {
			continue
		}
		var body struct {
			DispatchID string `json:"dispatchID"`
			MessageID  string `json:"messageID"`
			ReplyKind  string `json:"replyKind"`
		}
		if json.Unmarshal([]byte(event.Body), &body) != nil || body.ReplyKind != expectedReplyKind || body.DispatchID == "" {
			continue
		}
		if sourceReference == "" || body.MessageID == sourceReference || body.MessageID == messageID {
			return body.DispatchID
		}
	}
	return ""
}

func (session *mattermostScenarioSession) downloadAttachments(contextValue context.Context, fileIDs []string) ([]downloadedMattermostFile, error) {
	files := make([]downloadedMattermostFile, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		metadata, errorValue := session.mattermost.FileMetadata(contextValue, session.adminToken, fileID)
		if errorValue != nil {
			return files, errorValue
		}
		document, errorValue := session.mattermost.DownloadFile(contextValue, session.adminToken, fileID)
		if errorValue != nil {
			return files, errorValue
		}
		if int64(len(document)) != metadata.Size {
			return files, errors.New("Mattermost attachment size does not match its metadata")
		}
		files = append(files, downloadedMattermostFile{
			FileID:        fileID,
			Filename:      metadata.Name,
			ContentType:   metadata.MimeType,
			ContentBase64: base64.StdEncoding.EncodeToString(document),
		})
	}
	return files, nil
}

func (session *mattermostScenarioSession) execution() mattermostScenarioExecution {
	return mattermostScenarioExecution{
		Result:      session.result,
		Username:    session.username,
		Password:    session.password,
		RootPostID:  session.rootPostID,
		BotUsername: session.botUsername,
	}
}

func (session *mattermostScenarioSession) cleanup(contextValue context.Context) error {
	session.cleanupMutex.Lock()
	defer session.cleanupMutex.Unlock()
	if session.isCleanupFinished {
		return nil
	}
	var cleanupErrors []error
	if errorValue := session.admin.cleanup(contextValue, session.result, session.email); errorValue != nil {
		cleanupErrors = append(cleanupErrors, errorValue)
	}
	if session.adminToken != "" {
		for _, post := range session.result.Posts {
			if errorValue := session.mattermost.DeletePost(contextValue, session.adminToken, post.ID); errorValue != nil {
				cleanupErrors = append(cleanupErrors, errorValue)
			}
		}
	}
	if session.userID != "" && session.adminToken != "" {
		if errorValue := session.mattermost.DeleteUser(contextValue, session.adminToken, session.userID); errorValue != nil {
			cleanupErrors = append(cleanupErrors, errorValue)
		}
	}
	if len(cleanupErrors) == 0 {
		session.isCleanupFinished = true
	}
	return errors.Join(cleanupErrors...)
}

func mattermostScenarioEventIDSet(events []mattermostScenarioTaskEvent) map[string]bool {
	identifiers := make(map[string]bool, len(events))
	for _, event := range events {
		if event.TaskEventID != "" {
			identifiers[event.TaskEventID] = true
		}
	}
	return identifiers
}

func mattermostScenarioNewEvents(events []mattermostScenarioTaskEvent, previousEventIDs map[string]bool) []mattermostScenarioTaskEvent {
	newEvents := make([]mattermostScenarioTaskEvent, 0, len(events))
	for _, event := range events {
		if !previousEventIDs[event.TaskEventID] {
			newEvents = append(newEvents, event)
		}
	}
	return newEvents
}

func isMattermostScenarioTaskInProgress(status string) bool {
	switch status {
	case "", "planned", "pending", "created", "queued", "running":
		return true
	default:
		return false
	}
}

func convertMattermostScenarioPost(post mattermostProbePost) mattermostScenarioPost {
	return mattermostScenarioPost{
		ID:        post.ID,
		RootID:    post.RootID,
		UserID:    post.UserID,
		Message:   post.Message,
		FileIDs:   post.FileIDs,
		CreatedAt: post.CreatedAt,
	}
}

func findMattermostScenarioPublicURL(message string) string {
	for _, candidate := range mattermostScenarioURLPattern.FindAllString(message, -1) {
		trimmedCandidate := strings.TrimRight(candidate, ".,;:!?*]}'\"")
		if strings.Contains(trimmedCandidate, "intern.kim") {
			return trimmedCandidate
		}
	}
	return ""
}

func findMattermostScenarioEventPublicURL(events []mattermostScenarioTaskEvent) string {
	for eventIndex := len(events) - 1; eventIndex >= 0; eventIndex-- {
		event := events[eventIndex]
		if _, isDirectToolEvent := mattermostScenarioDirectToolName(event.Name, "result"); !isDirectToolEvent {
			continue
		}
		if publicURL := findMattermostScenarioPublicURL(event.Body); publicURL != "" {
			return publicURL
		}
	}
	return ""
}

func waitForMattermostScenarioPoll(contextValue context.Context) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-contextValue.Done():
		return contextValue.Err()
	case <-timer.C:
		return nil
	}
}
