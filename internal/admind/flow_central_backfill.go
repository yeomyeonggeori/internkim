package admind

import (
	"context"
	"fmt"
)

// §2 says every device write reaches the central plane. Tasks written before the
// outbox existed never did: nothing queued them, and only an edit would. They are
// the rows a central read would lose outright, so they are queued once here and
// the drain carries them the ordinary way, adopting a row that already matches
// rather than making a second one.
func (service *Service) queueFlowTasksTheCentralPlaneNeverSaw(ctx context.Context) (int, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return 0, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
	SELECT id FROM flow_tasks
	WHERE id NOT IN (SELECT task_id FROM flow_central_identity)`)
	if errorValue != nil {
		return 0, errorValue
	}
	defer rows.Close()
	taskIDs := []string{}
	for rows.Next() {
		taskID := ""
		if errorValue := rows.Scan(&taskID); errorValue != nil {
			return 0, errorValue
		}
		taskIDs = append(taskIDs, taskID)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return 0, errorValue
	}

	for _, taskID := range taskIDs {
		if errorValue := enqueueFlowCentralWrite(ctx, database, taskID); errorValue != nil {
			return 0, errorValue
		}
	}
	return len(taskIDs), nil
}

func (service *Service) reportFlowCentralBackfill(ctx context.Context) sshRecoveryCommandResult {
	queued, errorValue := service.queueFlowTasksTheCentralPlaneNeverSaw(ctx)
	if errorValue != nil {
		return sshRecoveryCommandResult{
			Name:   "queue the tasks the central plane never saw",
			Status: "error",
			Output: errorValue.Error(),
		}
	}
	return sshRecoveryCommandResult{
		Name:   "queue the tasks the central plane never saw",
		Status: "ok",
		Output: fmt.Sprintf("%d queued", queued),
	}
}
