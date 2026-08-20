package admind

import (
	"context"
	"fmt"
	"strings"
)

const flowCentralHeldBackReportName = "list the flow tasks the queue stopped trying to carry"

func (service *Service) reportFlowCentralHeldBack(ctx context.Context) sshRecoveryCommandResult {
	entries, errorValue := service.readFlowCentralOutboxHeldBack(ctx)
	if errorValue != nil {
		return sshRecoveryCommandResult{
			Name:   flowCentralHeldBackReportName,
			Status: "error",
			Output: errorValue.Error(),
		}
	}
	return sshRecoveryCommandResult{
		Name:   flowCentralHeldBackReportName,
		Status: "ok",
		Output: flowCentralHeldBackReport(entries),
	}
}

func flowCentralHeldBackReport(entries []flowCentralOutboxEntry) string {
	if len(entries) == 0 {
		return "the queue is holding nothing back"
	}
	lines := []string{fmt.Sprintf("%d held back after %d attempts each", len(entries), flowCentralOutboxAttemptLimit)}
	for _, entry := range entries {
		lines = append(lines, fmt.Sprintf("  %s %s last tried %s: %s", entry.TaskID, entry.Intent, flowCentralHeldBackMoment(entry), entry.LastError))
	}
	return strings.Join(append(lines, "queue them again with `internkim recover ssh --action flow-central-backfill` once the reason above is gone"), "\n")
}

func flowCentralHeldBackMoment(entry flowCentralOutboxEntry) string {
	if strings.TrimSpace(entry.LastAttemptedAt) == "" {
		return "never"
	}
	return entry.LastAttemptedAt
}
