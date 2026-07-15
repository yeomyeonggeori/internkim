package admind

import (
	"context"
	"database/sql"
	"fmt"
)

type flowSummaryCachePurgeExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func clearFlowSummaryCacheEntries(ctx context.Context, executor flowSummaryCachePurgeExecutor) error {
	if _, errorValue := executor.ExecContext(ctx, "DELETE FROM flow_summary_cache_entries"); errorValue != nil {
		return fmt.Errorf("clear flow summary cache entries: %w", errorValue)
	}
	return nil
}
