package admind

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestReadExpiredFlowMattermostPostTaskIDsFiltersByAge(t *testing.T) {
	service := NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	ctx := context.Background()

	oldTask := flowNotificationTestTask("완료")
	oldTask.ID = "old-task"
	if errorValue := service.writeFlowTask(ctx, oldTask); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.updateFlowTaskMattermostPostID(ctx, oldTask.ID, "old-post"); errorValue != nil {
		t.Fatal(errorValue)
	}
	backdateFlowMattermostPostCreatedAt(t, service, oldTask.ID, time.Now().UTC().Add(-20*24*time.Hour))

	freshTask := flowNotificationTestTask("완료")
	freshTask.ID = "fresh-task"
	if errorValue := service.writeFlowTask(ctx, freshTask); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.updateFlowTaskMattermostPostID(ctx, freshTask.ID, "fresh-post"); errorValue != nil {
		t.Fatal(errorValue)
	}

	unpostedTask := flowNotificationTestTask("요청")
	unpostedTask.ID = "unposted-task"
	if errorValue := service.writeFlowTask(ctx, unpostedTask); errorValue != nil {
		t.Fatal(errorValue)
	}

	taskIDs, errorValue := service.readExpiredFlowMattermostPostTaskIDs(ctx, time.Now().UTC().Add(-mattermostChannelPostRetentionDuration))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(taskIDs) != 1 || taskIDs[0] != "old-task" {
		t.Fatalf("expected only old-task, got %+v", taskIDs)
	}
}

func backdateFlowMattermostPostCreatedAt(t *testing.T, service *Service, taskID string, postCreatedAt time.Time) {
	t.Helper()
	database, errorValue := service.openFlowDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.Exec("UPDATE flow_tasks SET mattermost_post_created_at = ? WHERE id = ?", postCreatedAt.Format(time.RFC3339), taskID); errorValue != nil {
		t.Fatal(errorValue)
	}
}
