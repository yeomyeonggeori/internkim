package admind

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const (
	taskNotifyInterval    = time.Minute
	taskNotifyBatch       = 200
	taskNotifyMarkLife    = 30 * 24 * time.Hour
	taskNotifyPlatform    = "mattermost"
	taskNotifyListPath    = "/admin/api/task?viewerIsAdmin=true&limit=200"
	taskNotifyDetailPath  = "/admin/api/task/detail?viewerIsAdmin=true&taskRunID="
	taskNotifyConfirmName = "confirmation.requested"
)

// blueclaw has no way to call out when a run changes, so the device reads the
// same task list its dashboard reads and notices the transitions itself. The
// alternative would teach blueclaw a central-plane address it has no business
// knowing.
func (service *Service) keepTaskRunsNotified(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(taskNotifyInterval):
		}
		service.notifyTaskRunsOnce(ctx)
	}
}

func (service *Service) notifyTaskRunsOnce(ctx context.Context) {
	client := service.centralPlane()
	if client == nil {
		return
	}
	runs, errorValue := service.taskRunsToConsider(ctx)
	if errorValue != nil {
		log.Printf("task notify: the task list is unreadable: %v", errorValue)
		return
	}

	seeded, errorValue := service.taskNotifyBaselineSeeded(ctx)
	if errorValue != nil {
		log.Printf("task notify: the mark store is unreadable: %v", errorValue)
		return
	}
	now := time.Now()
	if !seeded {
		service.adoptTaskRunsWithoutNotifying(ctx, runs, now)
		return
	}

	marks, errorValue := service.readTaskNotifyMarks(ctx)
	if errorValue != nil {
		log.Printf("task notify: the marks are unreadable: %v", errorValue)
		return
	}
	service.notifyChangedTaskRuns(ctx, client, runs, marks, now)
	if errorValue := service.forgetStaleTaskNotifyMarks(ctx, now.Add(-taskNotifyMarkLife)); errorValue != nil {
		log.Printf("task notify: stale marks were not cleared: %v", errorValue)
	}
}

// The first cycle adopts whatever the ledger already holds. Without this a
// device that has been running for months would announce every finished task
// at once the moment this shipped.
func (service *Service) adoptTaskRunsWithoutNotifying(ctx context.Context, runs []taskNotifyRun, now time.Time) {
	adopted := map[string]string{}
	for _, run := range runs {
		adopted[run.TaskRunID] = run.Status
	}
	if errorValue := service.writeTaskNotifyMarks(ctx, adopted, now); errorValue != nil {
		log.Printf("task notify: the baseline was not recorded: %v", errorValue)
		return
	}
	if errorValue := service.writeTaskNotifyBaseline(ctx, now); errorValue != nil {
		log.Printf("task notify: the baseline marker was not recorded: %v", errorValue)
		return
	}
	log.Printf("task notify: adopted %d runs without notifying", len(adopted))
}

func (service *Service) notifyChangedTaskRuns(
	ctx context.Context,
	client *centralplane.Client,
	runs []taskNotifyRun,
	marks map[string]string,
	now time.Time,
) {
	externalIDByPersonID := taskNotifyExternalIDByPersonID(service.taskNotifyDirectoryRecords(ctx))
	for _, run := range runs {
		if marks[run.TaskRunID] == run.Status {
			continue
		}
		category, notifiable := taskNotifyCategory(run.Status)
		if !notifiable {
			service.markTaskRun(ctx, run, now)
			continue
		}
		externalID := externalIDByPersonID[run.RequesterPersonID]
		if externalID == "" {
			log.Printf("task notify: nobody here answers to person %q, so run %s is silent",
				run.RequesterPersonID, run.TaskRunID)
			service.markTaskRun(ctx, run, now)
			continue
		}
		notification := taskNotifyContent(run, category, service.taskNotifyConfirmationMessage(ctx, run))
		notification.Platform = taskNotifyPlatform
		notification.ExternalIDs = []string{externalID}

		result, errorValue := client.Notify(ctx, notification)
		if errorValue != nil {
			log.Printf("task notify: run %s (%s) was not sent: %v", run.TaskRunID, run.Status, errorValue)
			continue
		}
		log.Printf("task notify: run %s is %s; told=%d reached=%d", run.TaskRunID, run.Status, result.Told, result.Reached)
		service.markTaskRun(ctx, run, now)
	}
}

// A mark is written only once the plane has answered, so a send that failed is
// retried on the next tick rather than lost.
func (service *Service) markTaskRun(ctx context.Context, run taskNotifyRun, now time.Time) {
	if errorValue := service.writeTaskNotifyMarks(ctx, map[string]string{run.TaskRunID: run.Status}, now); errorValue != nil {
		log.Printf("task notify: run %s was not marked: %v", run.TaskRunID, errorValue)
	}
}

func (service *Service) taskRunsToConsider(ctx context.Context) ([]taskNotifyRun, error) {
	var runs []taskNotifyRun
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, taskNotifyListPath, nil, &runs); errorValue != nil {
		return nil, errorValue
	}
	if len(runs) > taskNotifyBatch {
		runs = runs[:taskNotifyBatch]
	}
	return runs, nil
}

// The approval wording the agent wrote for this person. It is an enrichment:
// when the detail call fails the notification still goes, carrying the prompt.
func (service *Service) taskNotifyConfirmationMessage(ctx context.Context, run taskNotifyRun) string {
	if run.Status != "waiting_approval" {
		return ""
	}
	var detail struct {
		TaskEvents []struct {
			Name string          `json:"name"`
			Body json.RawMessage `json:"body"`
		} `json:"taskEvents"`
	}
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, taskNotifyDetailPath+run.TaskRunID, nil, &detail); errorValue != nil {
		return ""
	}
	message := ""
	for _, event := range detail.TaskEvents {
		if event.Name != taskNotifyConfirmName {
			continue
		}
		var body struct {
			UserFacingMessage string `json:"userFacingMessage"`
		}
		if errorValue := json.Unmarshal(event.Body, &body); errorValue == nil && body.UserFacingMessage != "" {
			message = body.UserFacingMessage
		}
	}
	return message
}

func (service *Service) taskNotifyDirectoryRecords(ctx context.Context) []adminUserMutation {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	if errorValue != nil {
		return nil
	}
	return service.accountDirectoryUserRecords(request)
}
