package admind

import (
	"context"
	"database/sql"
	"fmt"
)

type taskSummaryCachePurgeExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func clearTaskSummaryCacheEntries(ctx context.Context, executor taskSummaryCachePurgeExecutor) error {
	if _, errorValue := executor.ExecContext(ctx, "DELETE FROM flow_summary_cache_entries"); errorValue != nil {
		return fmt.Errorf("clear flow summary cache entries: %w", errorValue)
	}
	return nil
}
