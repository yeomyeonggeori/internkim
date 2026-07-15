package cli

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeMattermostProbeAPI struct {
	postMessage       func(mattermostProbeMessage) mattermostProbePost
	listChannelPosts  func() []mattermostProbePost
	fileMetadata      map[string]mattermostProbeFileMetadata
	fileContents      map[string][]byte
	deletedChannelIDs []string
	deletedUserIDs    []string
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
	return fake.listChannelPosts(), nil
}

func (fake *fakeMattermostProbeAPI) FileMetadata(_ context.Context, _ string, fileID string) (mattermostProbeFileMetadata, error) {
	return fake.fileMetadata[fileID], nil
}

func (fake *fakeMattermostProbeAPI) DownloadFile(_ context.Context, _ string, fileID string) ([]byte, error) {
	return fake.fileContents[fileID], nil
}

func (fake *fakeMattermostProbeAPI) DeleteChannel(_ context.Context, _ string, channelID string) error {
	fake.deletedChannelIDs = append(fake.deletedChannelIDs, channelID)
	return nil
}

func (fake *fakeMattermostProbeAPI) DeleteUserPermanently(_ context.Context, _ string, userID string) error {
	fake.deletedUserIDs = append(fake.deletedUserIDs, userID)
	return nil
}

type fakeMattermostScenarioAdminAPI struct {
	listTasksValue     func() []mattermostScenarioTaskSummary
	taskDetailValue    func(string) mattermostScenarioTaskDetail
	workspaceFileValue func(mattermostScenarioStep) []mattermostScenarioWorkspaceResult
	workspaceFileError error
	cleanupCount       int
}

func (fake *fakeMattermostScenarioAdminAPI) readSecret(context.Context, string) (string, error) {
	return "secret", nil
}

func (fake *fakeMattermostScenarioAdminAPI) invitePerson(context.Context, string, string, string) error {
	return nil
}

func (fake *fakeMattermostScenarioAdminAPI) listTasks(context.Context, string) ([]mattermostScenarioTaskSummary, error) {
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
	return nil
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
				events = append(events, newEvent)
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
	if len(session.result.Steps[0].TaskEvents) != 1 || session.result.Steps[0].TaskEvents[0].TaskEventID != "new" {
		t.Fatalf("unexpected step events: %#v", session.result.Steps[0].TaskEvents)
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
			events := []mattermostScenarioTaskEvent{{TaskEventID: "event-" + taskRunID, Name: "task.completed"}}
			if taskRunID == "task-1" {
				events = append(events, mattermostScenarioTaskEvent{TaskEventID: "site-" + taskRunID, Name: "tool.site.publish.result", Body: `{"url":"https://preview.example.test/site"}`})
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
		fileMetadata: map[string]mattermostProbeFileMetadata{"file": {ID: "file", Name: "report.docx", MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"}},
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

func TestMattermostScenarioPreservesTaskEvidenceWhenPollingFails(t *testing.T) {
	pollError := errors.New("poll failed")
	detailCalls := 0
	oldEvent := mattermostScenarioTaskEvent{TaskEventID: "old", Name: "task.created"}
	newEvent := mattermostScenarioTaskEvent{TaskEventID: "new", Name: "llm.call", Body: `{"provider":"sdkd"}`}
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue: func() []mattermostScenarioTaskSummary {
			return []mattermostScenarioTaskSummary{{TaskRunID: "task", UpdatedAt: "updated"}}
		},
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			detailCalls++
			events := []mattermostScenarioTaskEvent{oldEvent}
			if detailCalls > 1 {
				events = append(events, newEvent)
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
	if len(stepResult.TaskEvents) != 1 || stepResult.TaskEvents[0].TaskEventID != "new" {
		t.Fatalf("unexpected partial task events: %#v", stepResult.TaskEvents)
	}
	if stepResult.DurationMS <= 0 || len(session.result.Posts) != 1 {
		t.Fatalf("duration=%d posts=%#v", stepResult.DurationMS, session.result.Posts)
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
				TaskEvents: []mattermostScenarioTaskEvent{{TaskEventID: "completed", Name: "task.completed"}},
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
	if !errors.Is(errorValue, workspaceError) {
		t.Fatalf("expected workspace failure, got %v", errorValue)
	}
	stepResult := session.result.Steps[0]
	if stepResult.TaskRunID != "task" || stepResult.BotMessage != "완료했습니다." || len(stepResult.TaskEvents) != 1 {
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
				TaskEvents: []mattermostScenarioTaskEvent{{TaskEventID: "completed", Name: "task.completed"}},
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
	if session.result.DurationMS <= 0 {
		t.Fatalf("expected failed scenario duration, got %d", session.result.DurationMS)
	}
}

func TestMattermostScenarioUnexpectedTerminalStatusReturnsWithoutPolling(t *testing.T) {
	admin := &fakeMattermostScenarioAdminAPI{
		listTasksValue: func() []mattermostScenarioTaskSummary {
			return []mattermostScenarioTaskSummary{{TaskRunID: "task", UpdatedAt: "now"}}
		},
		taskDetailValue: func(string) mattermostScenarioTaskDetail {
			return mattermostScenarioTaskDetail{TaskRun: mattermostScenarioTaskRun{TaskRunID: "task", Status: "failed"}, TaskEvents: []mattermostScenarioTaskEvent{{TaskEventID: "failed", Name: "task.failed"}}}
		},
	}
	session := newTestMattermostScenarioSession(mattermostScenario{}, &fakeMattermostProbeAPI{}, admin)
	pollCount := 0
	session.poll = func(context.Context) error { pollCount++; return nil }

	detail, _, errorValue := session.waitForStepTask(context.Background(), map[string]mattermostScenarioTaskSnapshot{}, "completed")
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

	if errorValue := session.cleanup(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := session.cleanup(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if admin.cleanupCount != 1 || len(mattermost.deletedChannelIDs) != 1 || len(mattermost.deletedUserIDs) != 1 {
		t.Fatalf("admin=%d channels=%v users=%v", admin.cleanupCount, mattermost.deletedChannelIDs, mattermost.deletedUserIDs)
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

	_, _, errorValue := session.waitForStepTask(contextValue, nil, "completed")
	if !errors.Is(errorValue, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", errorValue)
	}
}
