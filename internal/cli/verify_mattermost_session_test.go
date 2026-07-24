package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeMattermostProbeAPI struct {
	postMessage       func(mattermostProbeMessage) mattermostProbePost
	listChannelPosts  func() []mattermostProbePost
	fileMetadata      map[string]mattermostProbeFileMetadata
	fileContents      map[string][]byte
	deletedPostIDs    []string
	deletedUserIDs    []string
	deletePostError   error
	deleteUserError   error
	posts             map[string]mattermostProbePost
	getPostError      error
	clickedActions    []string
	doPostActionError error
}

func (fake *fakeMattermostProbeAPI) Login(context.Context, string, string) (string, error) {
	return "token", nil
}

func (fake *fakeMattermostProbeAPI) CurrentUser(context.Context, string) (mattermostProbeUser, error) {
	return mattermostProbeUser{ID: "bot"}, nil
}

func (fake *fakeMattermostProbeAPI) CreateUser(context.Context, string, mattermostProbeNewUser) (mattermostProbeUser, error) {
	return mattermostProbeUser{ID: "user"}, nil
}

func (fake *fakeMattermostProbeAPI) TeamByName(context.Context, string, string) (mattermostProbeTeam, error) {
	return mattermostProbeTeam{ID: "team"}, nil
}

func (fake *fakeMattermostProbeAPI) AddTeamMember(context.Context, string, string, string) error {
	return nil
}

func (fake *fakeMattermostProbeAPI) CreateDirectChannel(context.Context, string, string, string) (mattermostProbeChannel, error) {
	return mattermostProbeChannel{ID: "channel"}, nil
}

func (fake *fakeMattermostProbeAPI) PostMessage(_ context.Context, _ string, message mattermostProbeMessage) (mattermostProbePost, error) {
	return fake.postMessage(message), nil
}

func (fake *fakeMattermostProbeAPI) ListChannelPosts(context.Context, string, string) ([]mattermostProbePost, error) {
	if fake.listChannelPosts != nil {
		return fake.listChannelPosts(), nil
	}
	posts := []mattermostProbePost{}
	for _, post := range fake.posts {
		posts = append(posts, post)
	}
	return posts, nil
}

func (fake *fakeMattermostProbeAPI) GetPost(_ context.Context, _ string, postID string) (mattermostProbePost, error) {
	if fake.getPostError != nil {
		return mattermostProbePost{}, fake.getPostError
	}
	return fake.posts[postID], nil
}

func (fake *fakeMattermostProbeAPI) DoPostAction(_ context.Context, _ string, postID string, actionID string) error {
	if fake.doPostActionError != nil {
		return fake.doPostActionError
	}
	fake.clickedActions = append(fake.clickedActions, postID+":"+actionID)
	return nil
}

func (fake *fakeMattermostProbeAPI) FileMetadata(_ context.Context, _ string, fileID string) (mattermostProbeFileMetadata, error) {
	return fake.fileMetadata[fileID], nil
}

func (fake *fakeMattermostProbeAPI) DownloadFile(_ context.Context, _ string, fileID string) ([]byte, error) {
	return fake.fileContents[fileID], nil
}

func (fake *fakeMattermostProbeAPI) DeletePost(_ context.Context, _ string, postID string) error {
	fake.deletedPostIDs = append(fake.deletedPostIDs, postID)
	return fake.deletePostError
}

func (fake *fakeMattermostProbeAPI) DeleteUser(_ context.Context, _ string, userID string) error {
	fake.deletedUserIDs = append(fake.deletedUserIDs, userID)
	return fake.deleteUserError
}

type fakeMattermostScenarioAdminAPI struct {
	listTasksValue     func() []mattermostScenarioTaskSummary
	taskDetailValue    func(string) mattermostScenarioTaskDetail
	workspaceFileValue func(mattermostScenarioStep) []mattermostScenarioWorkspaceResult
	workspaceFileError error
	conversationIDs    []string
	cleanupCount       int
	cleanupError       error
}

func (fake *fakeMattermostScenarioAdminAPI) readSecret(context.Context, string) (string, error) {
	return "secret", nil
}

func (fake *fakeMattermostScenarioAdminAPI) invitePerson(context.Context, string, string, string) error {
	return nil
}

func (fake *fakeMattermostScenarioAdminAPI) listTasks(_ context.Context, conversationID string) ([]mattermostScenarioTaskSummary, error) {
	fake.conversationIDs = append(fake.conversationIDs, conversationID)
	return fake.listTasksValue(), nil
}

func (fake *fakeMattermostScenarioAdminAPI) taskDetail(_ context.Context, taskRunID string) (mattermostScenarioTaskDetail, error) {
	return fake.taskDetailValue(taskRunID), nil
}

func (fake *fakeMattermostScenarioAdminAPI) workspaceFiles(_ context.Context, step mattermostScenarioStep) ([]mattermostScenarioWorkspaceResult, error) {
	if fake.workspaceFileError != nil {
		return nil, fake.workspaceFileError
	}
	if fake.workspaceFileValue == nil {
		return nil, nil
	}
	return fake.workspaceFileValue(step), nil
}

func (fake *fakeMattermostScenarioAdminAPI) cleanup(context.Context, mattermostScenarioResult, string) error {
	fake.cleanupCount++
	return fake.cleanupError
}

func newTestMattermostScenarioSession(scenario mattermostScenario, mattermost mattermostProbeAPI, admin mattermostScenarioAdminAPI) *mattermostScenarioSession {
	session := newMattermostScenarioSession(scenario, mattermost, admin)
	session.adminToken = "admin-token"
	session.userToken = "user-token"
	session.botUserID = "bot"
	session.userID = "user"
	session.channelID = "channel"
	session.email = "probe@internkim.test"
	session.poll = func(context.Context) error { return errors.New("unexpected poll") }
	return session
}

func testMattermostSourceEvent(identifier string, postID string) mattermostScenarioTaskEvent {
	return mattermostScenarioTaskEvent{TaskEventID: identifier, Name: "agent.task_source", Body: `{"sourceReference":"mattermost:thread:channel:user-post:` + postID + `"}`}
}

func testMattermostLaunchedEvent(identifier string, postID string) mattermostScenarioTaskEvent {
	return mattermostScenarioTaskEvent{TaskEventID: identifier, Name: "agent.task_launched", Body: `{"sourceReference":"mattermost:thread:channel:user-post:` + postID + `"}`}
}

func testMattermostReplyEvent(identifier string, replyKind string, postID string, sourcePostID string) mattermostScenarioTaskEvent {
	return mattermostScenarioTaskEvent{TaskEventID: identifier, Name: "connector.reply.sent", Body: `{"dispatchID":"` + postID + `","messageID":"mattermost:thread:channel:user-post:` + sourcePostID + `","replyKind":"` + replyKind + `"}`}
}

func TestMattermostScenarioApprovalFollowupValidatesOnlyEventsCreatedAfterSnapshot(t *testing.T) {
	oldEvent := mattermostScenarioTaskEvent{TaskEventID: "old", Name: "tool.calendar.delete.requested"}
	newEvent := mattermostScenarioTaskEvent{TaskEventID: "new", Name: "approval.granted"}
	detailCalls := 0
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue: func() []mattermostScenarioTaskSummary {
			return []mattermostScenarioTaskSummary{{TaskRunID: "task", UpdatedAt: "updated"}}
		},
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			detailCalls++
			events := []mattermostScenarioTaskEvent{oldEvent}
			if detailCalls > 1 {
				events = append(events, testMattermostSourceEvent("source", "user-post"), newEvent, testMattermostReplyEvent("reply", "success", "bot-post", "user-post"))
			}
			return mattermostScenarioTaskDetail{TaskRun: mattermostScenarioTaskRun{TaskRunID: "task", Status: "completed"}, TaskEvents: events}
		},
	}
	mattermost := &fakeMattermostProbeAPI{
		postMessage: func(mattermostProbeMessage) mattermostProbePost {
			return mattermostProbePost{ID: "user-post", CreatedAt: 1}
		},
		listChannelPosts: func() []mattermostProbePost {
			return []mattermostProbePost{{ID: "bot-post", RootID: "user-post", UserID: "bot", Message: "수정했습니다.", CreatedAt: 2}}
		},
	}
	scenario := mattermostScenario{Steps: []mattermostScenarioStep{{Prompt: "삭제해줘", ExpectedEvents: []string{"approval.granted"}, ExpectedToolCallCounts: map[string]int{"calendar.delete": 0}, ExpectedTaskStatus: "completed"}}}
	session := newTestMattermostScenarioSession(scenario, mattermost, admin)

	if errorValue := session.runStep(context.Background(), 0); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(session.result.Steps[0].TaskEvents) != 3 || session.result.Steps[0].TaskEvents[1].TaskEventID != "new" {
		t.Fatalf("unexpected step events: %#v", session.result.Steps[0].TaskEvents)
	}
	expectedConversationIDs := []string{"channel", "thread:channel:user-post"}
	if len(admin.conversationIDs) != len(expectedConversationIDs) {
		t.Fatalf("unexpected conversation IDs: %#v", admin.conversationIDs)
	}
	for index := range expectedConversationIDs {
		if admin.conversationIDs[index] != expectedConversationIDs[index] {
			t.Fatalf("unexpected conversation IDs: %#v", admin.conversationIDs)
		}
	}
	if session.result.ConversationID != "thread:channel:user-post" {
		t.Fatalf("unexpected result conversation ID: %q", session.result.ConversationID)
	}
}

func TestMattermostScenarioHookReceivesEvidenceBeforeLaterStep(t *testing.T) {
	postedStep := 0
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue: func() []mattermostScenarioTaskSummary {
			if postedStep == 0 {
				return nil
			}
			tasks := []mattermostScenarioTaskSummary{{TaskRunID: "task-1", UpdatedAt: "1"}}
			if postedStep == 2 {
				tasks = append(tasks, mattermostScenarioTaskSummary{TaskRunID: "task-2", UpdatedAt: "2"})
			}
			return tasks
		},
		taskDetailValue: func(taskRunID string) mattermostScenarioTaskDetail {
			stepNumber := "1"
			botPostID := "bot-post-1"
			if taskRunID == "task-2" {
				stepNumber = "2"
				botPostID = "bot-post-2"
			}
			events := []mattermostScenarioTaskEvent{
				{TaskEventID: "source-" + taskRunID, Name: "agent.task_source", Body: `{"sourceReference":"mattermost:thread:channel:user-post-1:user-post-` + stepNumber + `"}`},
				{TaskEventID: "event-" + taskRunID, Name: "task.completed"},
				{TaskEventID: "reply-" + taskRunID, Name: "connector.reply.sent", Body: `{"dispatchID":"` + botPostID + `","messageID":"mattermost:thread:channel:user-post-1:user-post-` + stepNumber + `","replyKind":"success"}`},
			}
			if taskRunID == "task-1" {
				events = append(events, mattermostScenarioTaskEvent{TaskEventID: "site-" + taskRunID, Name: "tool.site.serve.result", Body: `{"url":"https://preview.example.test/site"}`})
			}
			return mattermostScenarioTaskDetail{
				TaskRun:    mattermostScenarioTaskRun{TaskRunID: taskRunID, Status: "completed"},
				TaskEvents: events,
			}
		},
	}
	mattermost := &fakeMattermostProbeAPI{
		postMessage: func(message mattermostProbeMessage) mattermostProbePost {
			postedStep++
			return mattermostProbePost{ID: "user-post-" + string(rune('0'+postedStep)), RootID: message.RootID, CreatedAt: int64(postedStep * 10)}
		},
		listChannelPosts: func() []mattermostProbePost {
			if postedStep == 1 {
				return []mattermostProbePost{{ID: "bot-post-1", RootID: "user-post-1", UserID: "bot", Message: "https://preview.example.test/site", FileIDs: []string{"file"}, CreatedAt: 11}}
			}
			return []mattermostProbePost{{ID: "bot-post-2", RootID: "user-post-1", UserID: "bot", Message: "삭제했습니다.", CreatedAt: 21}}
		},
		fileMetadata: map[string]mattermostProbeFileMetadata{"file": {ID: "file", Name: "report.docx", MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", Size: 8}},
		fileContents: map[string][]byte{"file": []byte("document")},
	}
	scenario := mattermostScenario{Steps: []mattermostScenarioStep{{Prompt: "만들어줘", ExpectedTaskStatus: "completed"}, {Prompt: "삭제해줘", ExpectedTaskStatus: "completed"}}}
	session := newTestMattermostScenarioSession(scenario, mattermost, admin)
	observedEvidence := false

	errorValue := session.run(context.Background(), func(_ context.Context, execution mattermostScenarioExecution, stepIndex int) error {
		if stepIndex == 0 {
			step := execution.Result.Steps[0]
			observedEvidence = step.PublicURL != "" && len(step.Attachments) == 1 && string(step.Attachments[0].ContentBase64) != ""
		}
		return nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !observedEvidence {
		t.Fatal("step hook did not receive public URL and attachment evidence")
	}
}

func TestMattermostScenarioCorrelatesTaskAndFinalReplyToCurrentPost(t *testing.T) {
	phase := 0
	oldSource := `{"sourceReference":"mattermost:thread:channel:root-post:user-post-1"}`
	newSource := `{"sourceReference":"mattermost:thread:channel:root-post:user-post-2"}`
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue: func() []mattermostScenarioTaskSummary {
			tasks := []mattermostScenarioTaskSummary{{TaskRunID: "old-task", UpdatedAt: "1"}}
			if phase > 0 {
				tasks[0].UpdatedAt = "2"
			}
			if phase > 1 {
				tasks = append(tasks, mattermostScenarioTaskSummary{TaskRunID: "new-task", UpdatedAt: "3"})
			}
			return tasks
		},
		taskDetailValue: func(taskRunID string) mattermostScenarioTaskDetail {
			if taskRunID == "old-task" {
				events := []mattermostScenarioTaskEvent{{TaskEventID: "old-source", Name: "agent.task_source", Body: oldSource}, {TaskEventID: "old-completed", Name: "task.completed"}}
				if phase > 0 {
					events = append(events, mattermostScenarioTaskEvent{TaskEventID: "old-reply", Name: "connector.reply.sent", Body: `{"dispatchID":"old-bot-post","messageID":"mattermost:thread:channel:root-post:user-post-1","replyKind":"success"}`})
				}
				return mattermostScenarioTaskDetail{TaskRun: mattermostScenarioTaskRun{TaskRunID: taskRunID, Status: "completed"}, TaskEvents: events}
			}
			return mattermostScenarioTaskDetail{
				TaskRun: mattermostScenarioTaskRun{TaskRunID: taskRunID, Status: "completed"},
				TaskEvents: []mattermostScenarioTaskEvent{
					{TaskEventID: "new-source", Name: "agent.task_source", Body: newSource},
					{TaskEventID: "new-completed", Name: "task.completed"},
					{TaskEventID: "new-reply", Name: "connector.reply.sent", Body: `{"dispatchID":"new-bot-post","messageID":"mattermost:thread:channel:root-post:user-post-2","replyKind":"success"}`},
				},
			}
		},
	}
	mattermost := &fakeMattermostProbeAPI{
		postMessage: func(message mattermostProbeMessage) mattermostProbePost {
			phase = 1
			return mattermostProbePost{ID: "user-post-2", RootID: message.RootID, UserID: "user", CreatedAt: 20}
		},
		listChannelPosts: func() []mattermostProbePost {
			return []mattermostProbePost{
				{ID: "old-bot-post", RootID: "root-post", UserID: "bot", Message: "이전 요청을 완료했습니다.", CreatedAt: 21},
				{ID: "new-bot-post", RootID: "root-post", UserID: "bot", Message: "현재 요청을 완료했습니다.", CreatedAt: 22},
			}
		},
	}
	session := newTestMattermostScenarioSession(mattermostScenario{Steps: []mattermostScenarioStep{{Prompt: "현재 업무를 처리해줘", ExpectedTaskStatus: "completed"}}}, mattermost, admin)
	session.rootPostID = "root-post"
	session.poll = func(context.Context) error {
		phase = 2
		return nil
	}

	if errorValue := session.runStep(context.Background(), 0); errorValue != nil {
		t.Fatal(errorValue)
	}
	result := session.result.Steps[0]
	if result.TaskRunID != "new-task" || result.BotMessage != "현재 요청을 완료했습니다." {
		t.Fatalf("current post received mismatched evidence: %#v", result)
	}
	if session.seenBotPostIDs["old-bot-post"] {
		t.Fatal("delayed previous reply was consumed by the current step")
	}
}

func TestMattermostScenarioAutoConfirmationFinishesSameTask(t *testing.T) {
	isApproved := false
	listCalls := 0
	initialEvent := mattermostScenarioTaskEvent{TaskEventID: "requested", Name: "confirmation.requested"}
	completedEvent := mattermostScenarioTaskEvent{TaskEventID: "executed", Name: "approval.executed"}
	sourceEvent := testMattermostSourceEvent("source", "user-post")
	approvalReplyEvent := testMattermostReplyEvent("approval-reply", "user_notice", "approval-post", "user-post")
	completedReplyEvent := testMattermostReplyEvent("completed-reply", "success", "completed-post", "user-post")
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue: func() []mattermostScenarioTaskSummary {
			listCalls++
			if listCalls == 1 {
				return nil
			}
			return []mattermostScenarioTaskSummary{{TaskRunID: "task", UpdatedAt: "updated"}}
		},
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			if !isApproved {
				return mattermostScenarioTaskDetail{TaskRun: mattermostScenarioTaskRun{TaskRunID: "task", Status: "waiting_approval"}, TaskEvents: []mattermostScenarioTaskEvent{sourceEvent, initialEvent, approvalReplyEvent}}
			}
			return mattermostScenarioTaskDetail{TaskRun: mattermostScenarioTaskRun{TaskRunID: "task", Status: "completed"}, TaskEvents: []mattermostScenarioTaskEvent{sourceEvent, initialEvent, approvalReplyEvent, completedEvent, completedReplyEvent}}
		},
	}
	mattermost := &fakeMattermostProbeAPI{
		postMessage: func(mattermostProbeMessage) mattermostProbePost {
			return mattermostProbePost{ID: "user-post", CreatedAt: 1}
		},
		listChannelPosts: func() []mattermostProbePost {
			posts := []mattermostProbePost{{ID: "approval-post", RootID: "user-post", UserID: "bot", Message: "승인이 필요합니다.", CreatedAt: 2}}
			if isApproved {
				posts = append(posts, mattermostProbePost{ID: "completed-post", RootID: "user-post", UserID: "bot", Message: "삭제했습니다.", CreatedAt: 3})
			}
			return posts
		},
	}
	scenario := mattermostScenario{Name: "approval", Steps: []mattermostScenarioStep{{
		Prompt:             "삭제해줘",
		ApprovalAction:     mattermostScenarioApprovalApprove,
		ExpectedEvents:     []string{"confirmation.requested", "approval.executed"},
		ExpectedTaskStatus: "completed",
	}}}
	session := newTestMattermostScenarioSession(scenario, mattermost, admin)
	session.shouldAutoConfirm = true

	errorValue := session.run(context.Background(), func(_ context.Context, execution mattermostScenarioExecution, _ int) error {
		if execution.Result.Steps[0].TaskStatus != "waiting_approval" {
			t.Fatalf("hook received status %q", execution.Result.Steps[0].TaskStatus)
		}
		isApproved = true
		return nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := session.result.Steps[0]
	if result.TaskRunID != "task" || result.TaskStatus != "completed" || result.BotPostID != "completed-post" || result.BotMessage != "삭제했습니다." {
		t.Fatalf("unexpected approval result: %#v", result)
	}
	if len(result.TaskEvents) != 5 || len(session.result.Posts) != 3 {
		t.Fatalf("events=%#v posts=%#v", result.TaskEvents, session.result.Posts)
	}
}

func testMattermostApprovalProps() mattermostProbePostProps {
	return mattermostProbePostProps{Attachments: []mattermostProbePostAttachment{{
		Actions: []mattermostProbePostAction{{ID: "askConfirm", Name: "확인", Type: "button"}, {ID: "askCancel", Name: "취소", Type: "button"}},
	}}}
}

func TestMattermostScenarioClickPendingApprovalClicksTheApproveAction(t *testing.T) {
	admin := &fakeMattermostScenarioAdminAPI{}
	mattermost := &fakeMattermostProbeAPI{
		posts: map[string]mattermostProbePost{"approval-post": {ID: "approval-post", UserID: "bot", Props: testMattermostApprovalProps()}},
		listChannelPosts: func() []mattermostProbePost {
			return []mattermostProbePost{
				{ID: "notice-post", UserID: "bot", CreatedAt: 1},
				{ID: "approval-post", UserID: "bot", CreatedAt: 2},
			}
		},
	}
	scenario := mattermostScenario{Steps: []mattermostScenarioStep{{Prompt: "삭제해줘", ApprovalAction: mattermostScenarioApprovalApprove}}}
	session := newTestMattermostScenarioSession(scenario, mattermost, admin)
	session.result.Steps = []mattermostScenarioStepResult{{Prompt: "삭제해줘", BotPostID: "notice-post"}}

	if errorValue := session.clickPendingApproval(context.Background(), 0); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(mattermost.clickedActions) != 1 || mattermost.clickedActions[0] != "approval-post:askConfirm" {
		t.Fatalf("expected the approve action to be clicked once, got %v", mattermost.clickedActions)
	}

	if errorValue := session.clickPendingApproval(context.Background(), 0); errorValue != nil && !strings.Contains(errorValue.Error(), "no pending Mattermost approval post") {
		t.Fatal(errorValue)
	}
	if len(mattermost.clickedActions) != 1 {
		t.Fatalf("expected an already-clicked post not to be clicked again, got %v", mattermost.clickedActions)
	}
}

func TestMattermostScenarioClickPendingApprovalSkipsStepsWithoutApproval(t *testing.T) {
	admin := &fakeMattermostScenarioAdminAPI{}
	mattermost := &fakeMattermostProbeAPI{}
	scenario := mattermostScenario{Steps: []mattermostScenarioStep{{Prompt: "업무 요청"}}}
	session := newTestMattermostScenarioSession(scenario, mattermost, admin)
	session.result.Steps = []mattermostScenarioStepResult{{Prompt: "업무 요청", BotPostID: "bot-post"}}

	if errorValue := session.clickPendingApproval(context.Background(), 0); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(mattermost.clickedActions) != 0 {
		t.Fatalf("expected no click for a step without an approval action, got %v", mattermost.clickedActions)
	}
}

func TestWaitForApprovalCompletionClicksEachNewConfirmationPromptInFastMode(t *testing.T) {
	sourceEvent := testMattermostSourceEvent("source", "user-post")
	initialAsk := mattermostScenarioTaskEvent{TaskEventID: "ask-1", Name: "confirmation.requested"}
	firstReAskEvent := mattermostScenarioTaskEvent{TaskEventID: "ask-2", Name: "confirmation.requested"}
	secondReAskEvent := mattermostScenarioTaskEvent{TaskEventID: "ask-3", Name: "confirmation.requested"}
	completedEvent := mattermostScenarioTaskEvent{TaskEventID: "executed", Name: "approval.executed"}
	firstReAskReply := testMattermostReplyEvent("reply-1", "user_notice", "approval-post-1", "user-post")
	secondReAskReply := testMattermostReplyEvent("reply-2", "user_notice", "approval-post-2", "user-post")
	completedReply := testMattermostReplyEvent("reply-3", "success", "completed-post", "user-post")

	pollCount := 0
	admin := &fakeMattermostScenarioAdminAPI{
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			events := []mattermostScenarioTaskEvent{sourceEvent, initialAsk, firstReAskEvent, firstReAskReply}
			status := "waiting_approval"
			switch {
			case pollCount >= 2:
				events = append(events, secondReAskEvent, secondReAskReply, completedEvent, completedReply)
				status = "completed"
			case pollCount >= 1:
				events = append(events, secondReAskEvent, secondReAskReply)
			}
			return mattermostScenarioTaskDetail{TaskRun: mattermostScenarioTaskRun{TaskRunID: "task", Status: status}, TaskEvents: events}
		},
	}
	mattermost := &fakeMattermostProbeAPI{
		posts: map[string]mattermostProbePost{
			"approval-post-1": {ID: "approval-post-1", Props: testMattermostApprovalProps()},
			"approval-post-2": {ID: "approval-post-2", Props: testMattermostApprovalProps()},
		},
	}
	session := newTestMattermostScenarioSession(mattermostScenario{Steps: []mattermostScenarioStep{{Prompt: "삭제해줘"}}}, mattermost, admin)
	session.shouldRunFast = true
	session.poll = func(context.Context) error {
		pollCount++
		return nil
	}

	detail, _, replyPostID, errorValue := session.waitForApprovalCompletion(context.Background(), 0, "task", []mattermostScenarioTaskEvent{sourceEvent, initialAsk})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if detail.TaskRun.Status != "completed" || replyPostID != "completed-post" {
		t.Fatalf("unexpected completion: status=%q replyPostID=%q", detail.TaskRun.Status, replyPostID)
	}
	expectedClicks := []string{"approval-post-1:askConfirm", "approval-post-2:askConfirm"}
	if len(mattermost.clickedActions) != len(expectedClicks) || mattermost.clickedActions[0] != expectedClicks[0] || mattermost.clickedActions[1] != expectedClicks[1] {
		t.Fatalf("expected both re-ask prompts to be clicked in order, got %v", mattermost.clickedActions)
	}
}

func TestWaitForApprovalCompletionDoesNotClickWhenNotFast(t *testing.T) {
	sourceEvent := testMattermostSourceEvent("source", "user-post")
	initialAsk := mattermostScenarioTaskEvent{TaskEventID: "ask-1", Name: "confirmation.requested"}
	reAskEvent := mattermostScenarioTaskEvent{TaskEventID: "ask-2", Name: "confirmation.requested"}
	reAskReply := testMattermostReplyEvent("reply-1", "user_notice", "approval-post-1", "user-post")
	completedEvent := mattermostScenarioTaskEvent{TaskEventID: "executed", Name: "approval.executed"}
	completedReply := testMattermostReplyEvent("reply-2", "success", "completed-post", "user-post")

	pollCount := 0
	admin := &fakeMattermostScenarioAdminAPI{
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			if pollCount >= 1 {
				return mattermostScenarioTaskDetail{
					TaskRun:    mattermostScenarioTaskRun{TaskRunID: "task", Status: "completed"},
					TaskEvents: []mattermostScenarioTaskEvent{sourceEvent, initialAsk, reAskEvent, reAskReply, completedEvent, completedReply},
				}
			}
			return mattermostScenarioTaskDetail{
				TaskRun:    mattermostScenarioTaskRun{TaskRunID: "task", Status: "waiting_approval"},
				TaskEvents: []mattermostScenarioTaskEvent{sourceEvent, initialAsk, reAskEvent, reAskReply},
			}
		},
	}
	mattermost := &fakeMattermostProbeAPI{
		posts: map[string]mattermostProbePost{"approval-post-1": {ID: "approval-post-1", Props: testMattermostApprovalProps()}},
	}
	session := newTestMattermostScenarioSession(mattermostScenario{Steps: []mattermostScenarioStep{{Prompt: "삭제해줘"}}}, mattermost, admin)
	session.poll = func(context.Context) error {
		pollCount++
		return nil
	}

	detail, _, replyPostID, errorValue := session.waitForApprovalCompletion(context.Background(), 0, "task", []mattermostScenarioTaskEvent{sourceEvent, initialAsk})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if detail.TaskRun.Status != "completed" || replyPostID != "completed-post" {
		t.Fatalf("unexpected completion: status=%q replyPostID=%q", detail.TaskRun.Status, replyPostID)
	}
	if len(mattermost.clickedActions) != 0 {
		t.Fatalf("expected no REST clicks outside fast mode, got %v", mattermost.clickedActions)
	}
}

func TestMattermostScenarioApprovalRequiresAutoConfirmation(t *testing.T) {
	listCalls := 0
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue: func() []mattermostScenarioTaskSummary {
			listCalls++
			if listCalls == 1 {
				return nil
			}
			return []mattermostScenarioTaskSummary{{TaskRunID: "task", UpdatedAt: "updated"}}
		},
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			return mattermostScenarioTaskDetail{TaskRun: mattermostScenarioTaskRun{TaskRunID: "task", Status: "waiting_approval"}, TaskEvents: []mattermostScenarioTaskEvent{testMattermostSourceEvent("source", "user-post"), {TaskEventID: "requested", Name: "confirmation.requested"}, testMattermostReplyEvent("reply", "user_notice", "approval-post", "user-post")}}
		},
	}
	mattermost := &fakeMattermostProbeAPI{
		postMessage: func(mattermostProbeMessage) mattermostProbePost {
			return mattermostProbePost{ID: "user-post", CreatedAt: 1}
		},
		listChannelPosts: func() []mattermostProbePost {
			return []mattermostProbePost{{ID: "approval-post", RootID: "user-post", UserID: "bot", Message: "승인이 필요합니다.", CreatedAt: 2}}
		},
	}
	scenario := mattermostScenario{Name: "approval", Steps: []mattermostScenarioStep{{Prompt: "삭제해줘", ApprovalAction: mattermostScenarioApprovalApprove}}}
	session := newTestMattermostScenarioSession(scenario, mattermost, admin)
	hookCalled := false

	errorValue := session.run(context.Background(), func(context.Context, mattermostScenarioExecution, int) error {
		hookCalled = true
		return nil
	})
	if errorValue == nil || errorValue.Error() != "Mattermost scenario approval action requires --auto-confirm" {
		t.Fatalf("unexpected error: %v", errorValue)
	}
	if hookCalled {
		t.Fatal("approval hook ran without auto confirmation")
	}
}

func TestMattermostScenarioPreservesTaskEvidenceWhenPollingFails(t *testing.T) {
	pollError := errors.New("poll failed")
	detailCalls := 0
	oldEvent := mattermostScenarioTaskEvent{TaskEventID: "old", Name: "task.created"}
	newEvent := mattermostScenarioTaskEvent{TaskEventID: "new", Name: "llm.call", Body: `{"provider":"llmd"}`}
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue: func() []mattermostScenarioTaskSummary {
			return []mattermostScenarioTaskSummary{{TaskRunID: "task", UpdatedAt: "updated"}}
		},
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			detailCalls++
			events := []mattermostScenarioTaskEvent{oldEvent}
			if detailCalls > 1 {
				events = append(events, testMattermostSourceEvent("source", "user-post"), newEvent)
			}
			return mattermostScenarioTaskDetail{
				TaskRun:    mattermostScenarioTaskRun{TaskRunID: "task", Status: "running"},
				TaskEvents: events,
			}
		},
	}
	mattermost := &fakeMattermostProbeAPI{
		postMessage: func(mattermostProbeMessage) mattermostProbePost {
			return mattermostProbePost{ID: "user-post", UserID: "user", CreatedAt: 1}
		},
	}
	session := newTestMattermostScenarioSession(
		mattermostScenario{Steps: []mattermostScenarioStep{{Prompt: "업무를 처리해줘"}}},
		mattermost,
		admin,
	)
	session.poll = func(context.Context) error {
		time.Sleep(2 * time.Millisecond)
		return pollError
	}

	errorValue := session.runStep(context.Background(), 0)
	if !errors.Is(errorValue, pollError) {
		t.Fatalf("expected polling failure, got %v", errorValue)
	}
	if len(session.result.Steps) != 1 {
		t.Fatalf("expected partial step evidence, got %#v", session.result.Steps)
	}
	stepResult := session.result.Steps[0]
	if stepResult.TaskRunID != "task" || stepResult.TaskStatus != "running" {
		t.Fatalf("unexpected partial task evidence: %#v", stepResult)
	}
	if len(stepResult.TaskEvents) != 2 || stepResult.TaskEvents[1].TaskEventID != "new" {
		t.Fatalf("unexpected partial task events: %#v", stepResult.TaskEvents)
	}
	if stepResult.ProcessingMS <= 0 || len(session.result.Posts) != 1 {
		t.Fatalf("duration=%d posts=%#v", stepResult.ProcessingMS, session.result.Posts)
	}
}

func TestMattermostScenarioPreservesReplyEvidenceWhenWorkspaceInspectionFails(t *testing.T) {
	workspaceError := errors.New("workspace unavailable")
	listCalls := 0
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue: func() []mattermostScenarioTaskSummary {
			listCalls++
			if listCalls == 1 {
				return nil
			}
			return []mattermostScenarioTaskSummary{{TaskRunID: "task", UpdatedAt: "updated"}}
		},
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			return mattermostScenarioTaskDetail{
				TaskRun:    mattermostScenarioTaskRun{TaskRunID: "task", Status: "completed"},
				TaskEvents: []mattermostScenarioTaskEvent{testMattermostSourceEvent("source", "user-post"), {TaskEventID: "completed", Name: "task.completed"}, testMattermostReplyEvent("reply", "success", "bot-post", "user-post")},
			}
		},
		workspaceFileError: workspaceError,
	}
	mattermost := &fakeMattermostProbeAPI{
		postMessage: func(mattermostProbeMessage) mattermostProbePost {
			return mattermostProbePost{ID: "user-post", UserID: "user", CreatedAt: 1}
		},
		listChannelPosts: func() []mattermostProbePost {
			return []mattermostProbePost{{ID: "bot-post", RootID: "user-post", UserID: "bot", Message: "완료했습니다.", CreatedAt: 2}}
		},
	}
	session := newTestMattermostScenarioSession(
		mattermostScenario{Steps: []mattermostScenarioStep{{Prompt: "업무를 처리해줘"}}},
		mattermost,
		admin,
	)

	errorValue := session.runStep(context.Background(), 0)
	if errorValue != nil {
		t.Fatalf("expected blocked workspace evidence to record instead of aborting, got %v", errorValue)
	}
	stepResult := session.result.Steps[0]
	if !strings.Contains(stepResult.WorkspaceEvidenceError, "workspace unavailable") {
		t.Fatalf("expected the workspace failure recorded on the step, got %#v", stepResult)
	}
	if stepResult.TaskRunID != "task" || stepResult.BotMessage != "완료했습니다." || len(stepResult.TaskEvents) != 3 {
		t.Fatalf("unexpected partial reply evidence: %#v", stepResult)
	}
	if len(session.result.Posts) != 2 {
		t.Fatalf("expected user and bot posts, got %#v", session.result.Posts)
	}
}

func TestMattermostScenarioRecordsDurationWhenHookFails(t *testing.T) {
	hookError := errors.New("artifact verification failed")
	listCalls := 0
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue: func() []mattermostScenarioTaskSummary {
			listCalls++
			if listCalls == 1 {
				return nil
			}
			return []mattermostScenarioTaskSummary{{TaskRunID: "task", UpdatedAt: "updated"}}
		},
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			return mattermostScenarioTaskDetail{
				TaskRun:    mattermostScenarioTaskRun{TaskRunID: "task", Status: "completed"},
				TaskEvents: []mattermostScenarioTaskEvent{testMattermostSourceEvent("source", "user-post"), {TaskEventID: "completed", Name: "task.completed"}, testMattermostReplyEvent("reply", "success", "bot-post", "user-post")},
			}
		},
	}
	mattermost := &fakeMattermostProbeAPI{
		postMessage: func(mattermostProbeMessage) mattermostProbePost {
			return mattermostProbePost{ID: "user-post", UserID: "user", CreatedAt: 1}
		},
		listChannelPosts: func() []mattermostProbePost {
			return []mattermostProbePost{{ID: "bot-post", RootID: "user-post", UserID: "bot", Message: "완료했습니다.", CreatedAt: 2}}
		},
	}
	session := newTestMattermostScenarioSession(
		mattermostScenario{Name: "duration", Steps: []mattermostScenarioStep{{Prompt: "업무를 처리해줘", ExpectedTaskStatus: "completed"}}},
		mattermost,
		admin,
	)

	errorValue := session.run(context.Background(), func(context.Context, mattermostScenarioExecution, int) error {
		time.Sleep(2 * time.Millisecond)
		return hookError
	})
	if !errors.Is(errorValue, hookError) {
		t.Fatalf("expected hook failure, got %v", errorValue)
	}
	if session.result.ScenarioWallDurationMS <= 0 {
		t.Fatalf("expected failed scenario duration, got %d", session.result.ScenarioWallDurationMS)
	}
}

func TestMattermostScenarioUnexpectedTerminalStatusReturnsWithoutPolling(t *testing.T) {
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue: func() []mattermostScenarioTaskSummary {
			return []mattermostScenarioTaskSummary{{TaskRunID: "task", UpdatedAt: "now"}}
		},
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			return mattermostScenarioTaskDetail{TaskRun: mattermostScenarioTaskRun{TaskRunID: "task", Status: "failed"}, TaskEvents: []mattermostScenarioTaskEvent{testMattermostLaunchedEvent("source", "post"), {TaskEventID: "failed", Name: "task.failed"}, testMattermostReplyEvent("reply", "user_notice", "bot-post", "post")}}
		},
	}
	session := newTestMattermostScenarioSession(mattermostScenario{}, &fakeMattermostProbeAPI{}, admin)
	pollCount := 0
	session.poll = func(context.Context) error { pollCount++; return nil }

	detail, _, _, errorValue := session.waitForStepTask(context.Background(), 0, map[string]mattermostScenarioTaskSnapshot{}, "mattermost:thread:channel:user-post:post")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if detail.TaskRun.Status != "failed" || pollCount != 0 {
		t.Fatalf("status=%q pollCount=%d", detail.TaskRun.Status, pollCount)
	}
}

func TestMattermostScenarioCleanupIsIdempotent(t *testing.T) {
	mattermost := &fakeMattermostProbeAPI{}
	admin := &fakeMattermostScenarioAdminAPI{}
	session := newTestMattermostScenarioSession(mattermostScenario{}, mattermost, admin)
	session.result.Posts = []mattermostScenarioPost{{ID: "user-post"}, {ID: "bot-post"}}

	if errorValue := session.cleanup(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := session.cleanup(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if admin.cleanupCount != 1 || len(mattermost.deletedPostIDs) != 2 || len(mattermost.deletedUserIDs) != 1 {
		t.Fatalf("admin=%d posts=%v users=%v", admin.cleanupCount, mattermost.deletedPostIDs, mattermost.deletedUserIDs)
	}
}

func TestMattermostScenarioCleanupDeletesUndeletedMessageSideEffects(t *testing.T) {
	mattermost := &fakeMattermostProbeAPI{}
	admin := &fakeMattermostScenarioAdminAPI{}
	session := newTestMattermostScenarioSession(mattermostScenario{}, mattermost, admin)
	session.result.Steps = []mattermostScenarioStepResult{
		{TaskEvents: []mattermostScenarioTaskEvent{{Name: "tool.message.send.result", Body: `{"output":{"data":{"messageIDs":["sent-1","sent-2"],"deliveryStatus":"sent"}}}`}}},
		{TaskEvents: []mattermostScenarioTaskEvent{{Name: "tool.message.delete.result", Body: `{"output":{"data":{"messageIDs":["sent-2"],"deliveryStatus":"deleted"}}}`}}},
	}

	if errorValue := session.cleanup(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(mattermost.deletedPostIDs) != 1 || mattermost.deletedPostIDs[0] != "sent-1" {
		t.Fatalf("unexpected side-effect cleanup: %#v", mattermost.deletedPostIDs)
	}
}

func TestMattermostScenarioCleanupAttemptsEveryResourceAfterFailures(t *testing.T) {
	postError := errors.New("post cleanup failed")
	userError := errors.New("user cleanup failed")
	adminError := errors.New("workspace cleanup failed")
	mattermost := &fakeMattermostProbeAPI{deletePostError: postError, deleteUserError: userError}
	admin := &fakeMattermostScenarioAdminAPI{cleanupError: adminError}
	session := newTestMattermostScenarioSession(mattermostScenario{}, mattermost, admin)
	session.result.Posts = []mattermostScenarioPost{{ID: "user-post"}, {ID: "bot-post"}}

	errorValue := session.cleanup(context.Background())
	if !errors.Is(errorValue, adminError) || !errors.Is(errorValue, postError) || !errors.Is(errorValue, userError) {
		t.Fatalf("cleanup error = %v", errorValue)
	}
	if admin.cleanupCount != 1 || len(mattermost.deletedPostIDs) != 2 || len(mattermost.deletedUserIDs) != 1 {
		t.Fatalf("admin=%d posts=%v users=%v", admin.cleanupCount, mattermost.deletedPostIDs, mattermost.deletedUserIDs)
	}
}

func TestMattermostScenarioPollingStopsWhenContextIsCancelled(t *testing.T) {
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue:  func() []mattermostScenarioTaskSummary { return nil },
		taskDetailValue: func(string) mattermostScenarioTaskDetail { return mattermostScenarioTaskDetail{} },
	}
	session := newTestMattermostScenarioSession(mattermostScenario{}, &fakeMattermostProbeAPI{}, admin)
	session.poll = waitForMattermostScenarioPoll
	contextValue, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, _, errorValue := session.waitForStepTask(contextValue, 0, nil, "source")
	if !errors.Is(errorValue, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", errorValue)
	}
}

func TestNewMattermostScenarioSessionDefaultsToFifteenMinuteStepTimeout(t *testing.T) {
	session := newMattermostScenarioSession(mattermostScenario{}, &fakeMattermostProbeAPI{}, &fakeMattermostScenarioAdminAPI{})
	if session.stepTimeout != mattermostScenarioStepReplyTimeout {
		t.Fatalf("expected default step timeout of %s, got %s", mattermostScenarioStepReplyTimeout, session.stepTimeout)
	}
}

func TestMattermostScenarioWaitForStepTaskTimesOutWithoutProgress(t *testing.T) {
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue: func() []mattermostScenarioTaskSummary {
			return []mattermostScenarioTaskSummary{{TaskRunID: "task", UpdatedAt: "updated"}}
		},
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			return mattermostScenarioTaskDetail{
				TaskRun:    mattermostScenarioTaskRun{TaskRunID: "task", Status: "running"},
				TaskEvents: []mattermostScenarioTaskEvent{testMattermostSourceEvent("source", "post")},
			}
		},
	}
	session := newTestMattermostScenarioSession(mattermostScenario{}, &fakeMattermostProbeAPI{}, admin)
	session.stepTimeout = time.Millisecond
	session.poll = func(context.Context) error {
		time.Sleep(time.Millisecond)
		return nil
	}

	_, _, _, errorValue := session.waitForStepTask(context.Background(), 2, map[string]mattermostScenarioTaskSnapshot{}, "mattermost:thread:channel:user-post:post")
	if errorValue == nil {
		t.Fatal("expected a step timeout error")
	}
	if !strings.Contains(errorValue.Error(), "step 2") || !strings.Contains(errorValue.Error(), "task status") || !strings.Contains(errorValue.Error(), "--keep") {
		t.Fatalf("unexpected timeout error: %v", errorValue)
	}
}

func TestMattermostScenarioWaitForStepReplyTimesOutWithoutABotReply(t *testing.T) {
	mattermost := &fakeMattermostProbeAPI{listChannelPosts: func() []mattermostProbePost { return nil }}
	session := newTestMattermostScenarioSession(mattermostScenario{}, mattermost, &fakeMattermostScenarioAdminAPI{})
	session.stepTimeout = time.Millisecond
	session.poll = func(context.Context) error {
		time.Sleep(time.Millisecond)
		return nil
	}

	_, errorValue := session.waitForStepReply(context.Background(), 1, "missing-post")
	if errorValue == nil {
		t.Fatal("expected a step timeout error")
	}
	if !strings.Contains(errorValue.Error(), "step 1") || !strings.Contains(errorValue.Error(), "a bot reply") || !strings.Contains(errorValue.Error(), "--keep") {
		t.Fatalf("unexpected timeout error: %v", errorValue)
	}
}

func TestMattermostScenarioWaitForApprovalCompletionTimesOutWithoutProgress(t *testing.T) {
	admin := &fakeMattermostScenarioAdminAPI{
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			return mattermostScenarioTaskDetail{TaskRun: mattermostScenarioTaskRun{TaskRunID: "task", Status: "waiting_approval"}}
		},
	}
	session := newTestMattermostScenarioSession(mattermostScenario{}, &fakeMattermostProbeAPI{}, admin)
	session.stepTimeout = time.Millisecond
	session.poll = func(context.Context) error {
		time.Sleep(time.Millisecond)
		return nil
	}

	_, _, _, errorValue := session.waitForApprovalCompletion(context.Background(), 3, "task", nil)
	if errorValue == nil {
		t.Fatal("expected a step timeout error")
	}
	if !strings.Contains(errorValue.Error(), "step 3") || !strings.Contains(errorValue.Error(), "approval completion") || !strings.Contains(errorValue.Error(), "--keep") {
		t.Fatalf("unexpected timeout error: %v", errorValue)
	}
}

func TestMattermostScenarioStepMetricsDescribeAgentWork(t *testing.T) {
	result := mattermostScenarioStepResult{TaskEvents: []mattermostScenarioTaskEvent{
		{Name: "llm.call", Body: `{"schemaName":"blueclaw_turn_router"}`},
		{Name: "llm.call", Body: `{"schemaName":"blueclaw_agent_turn_action"}`},
		{Name: "agent.action"},
		{Name: "tool.task.add.requested"},
		{Name: "tool.capability.invoke.requested"},
	}}
	setMattermostScenarioStepMetrics(&result)
	if result.LLMCallCount != 2 || result.AgentStepCount != 1 || result.ToolCallCount != 1 {
		t.Fatalf("unexpected step metrics: %#v", result)
	}
}

func TestMattermostScenarioStepMetricsCountsNativeAgentActionsWithoutDoubleCounting(t *testing.T) {
	result := mattermostScenarioStepResult{TaskEvents: []mattermostScenarioTaskEvent{
		{Name: "llm.call", Body: `{"kind":"chat"}`},
		{Name: "agent.action"},
		{Name: "llm.call", Body: `{"kind":"structured","schemaName":"blueclaw_agent_turn_action"}`},
		{Name: "agent.action"},
	}}
	setMattermostScenarioStepMetrics(&result)
	if result.AgentStepCount != 2 {
		t.Fatalf("expected two native agent steps, got %#v", result)
	}
}

func TestMattermostScenarioStepMetricsAggregateTokenUsageAcrossLLMCalls(t *testing.T) {
	result := mattermostScenarioStepResult{TaskEvents: []mattermostScenarioTaskEvent{
		{Name: "llm.call", Body: `{"promptTokens":100,"completionTokens":20,"totalTokens":120,"cachedPromptTokens":40,"reasoningTokens":5,"costUSD":0.01}`},
		{Name: "llm.call", Body: `{"promptTokens":50,"completionTokens":10,"totalTokens":60,"cachedPromptTokens":10,"reasoningTokens":0,"costUSD":0.005}`},
		{Name: "agent.action"},
		{Name: "tool.task.add.requested"},
	}}
	setMattermostScenarioStepMetrics(&result)
	usage := result.TokenUsage
	if usage.LLMCallCount != 2 {
		t.Fatalf("expected two llm calls, got %#v", usage)
	}
	if usage.PromptTokens != 150 || usage.CompletionTokens != 30 || usage.TotalTokens != 180 {
		t.Fatalf("unexpected token totals: %#v", usage)
	}
	if usage.CachedPromptTokens != 50 || usage.ReasoningTokens != 5 {
		t.Fatalf("unexpected cache/reasoning totals: %#v", usage)
	}
	if usage.CostUSD != 0.015 {
		t.Fatalf("unexpected cost total: %#v", usage)
	}
	expectedHitRatio := 50.0 / 150.0
	if usage.CacheHitRatio != expectedHitRatio {
		t.Fatalf("expected cache hit ratio %v, got %v", expectedHitRatio, usage.CacheHitRatio)
	}
}

func TestMattermostScenarioTokenUsageGuardsZeroPromptDivision(t *testing.T) {
	result := mattermostScenarioStepResult{TaskEvents: []mattermostScenarioTaskEvent{
		{Name: "llm.call", Body: `{"completionTokens":20,"totalTokens":20}`},
	}}
	setMattermostScenarioStepMetrics(&result)
	if result.TokenUsage.PromptTokens != 0 || result.TokenUsage.CachedPromptTokens != 0 {
		t.Fatalf("expected zero prompt and cached tokens, got %#v", result.TokenUsage)
	}
	if result.TokenUsage.CacheHitRatio != 0 {
		t.Fatalf("expected cache hit ratio 0 when prompt tokens are zero, got %v", result.TokenUsage.CacheHitRatio)
	}
}

func TestMattermostScenarioTokenUsageTreatsOmittedFieldsAsZero(t *testing.T) {
	result := mattermostScenarioStepResult{TaskEvents: []mattermostScenarioTaskEvent{
		{Name: "llm.call", Body: `{"schemaName":"blueclaw_turn_router","transport":"llmd"}`},
		{Name: "agent.action"},
	}}
	setMattermostScenarioStepMetrics(&result)
	if result.TokenUsage.LLMCallCount != 1 {
		t.Fatalf("expected one llm call counted despite missing usage fields, got %#v", result.TokenUsage)
	}
	usage := result.TokenUsage
	if usage.PromptTokens != 0 || usage.CompletionTokens != 0 || usage.TotalTokens != 0 || usage.CachedPromptTokens != 0 || usage.ReasoningTokens != 0 || usage.CostUSD != 0 || usage.CacheHitRatio != 0 {
		t.Fatalf("expected all zero-value usage fields, got %#v", usage)
	}
}

func TestSumMattermostScenarioTokenUsageAggregatesStepsAndTokensPerStep(t *testing.T) {
	steps := []mattermostScenarioStepResult{
		{TaskEvents: []mattermostScenarioTaskEvent{
			{Name: "llm.call", Body: `{"promptTokens":100,"completionTokens":20,"totalTokens":120,"cachedPromptTokens":25,"costUSD":0.01}`},
		}},
		{TaskEvents: []mattermostScenarioTaskEvent{
			{Name: "llm.call", Body: `{"promptTokens":200,"completionTokens":40,"totalTokens":240,"cachedPromptTokens":75,"costUSD":0.02}`},
		}},
	}
	for stepIndex := range steps {
		setMattermostScenarioStepMetrics(&steps[stepIndex])
	}
	total := sumMattermostScenarioTokenUsage(steps)
	if total.LLMCallCount != 2 || total.PromptTokens != 300 || total.CompletionTokens != 60 || total.TotalTokens != 360 {
		t.Fatalf("unexpected scenario token totals: %#v", total)
	}
	if total.CachedPromptTokens != 100 || total.CostUSD != 0.03 {
		t.Fatalf("unexpected scenario cache/cost totals: %#v", total)
	}
	expectedHitRatio := 100.0 / 300.0
	if total.CacheHitRatio != expectedHitRatio {
		t.Fatalf("expected scenario cache hit ratio %v, got %v", expectedHitRatio, total.CacheHitRatio)
	}
	tokensPerStep := mattermostScenarioTokensPerStep(total, len(steps))
	if tokensPerStep != 180 {
		t.Fatalf("expected 180 tokens per step, got %v", tokensPerStep)
	}
	if mattermostScenarioTokensPerStep(total, 0) != 0 {
		t.Fatalf("expected tokens per step to guard against zero step count")
	}
}

func TestMattermostScenarioEventPublicURLRequiresDirectTypedResult(t *testing.T) {
	events := []mattermostScenarioTaskEvent{
		{Name: "tool.site.serve.result", Body: `{"publicURL":"https://demo.example.test"}`},
		{Name: "tool.capability.invoke.result", Body: `{"publicURL":"https://legacy.example.test"}`},
	}
	if publicURL := findMattermostScenarioEventPublicURL(events); publicURL != "https://demo.example.test" {
		t.Fatalf("public URL = %q", publicURL)
	}
}
