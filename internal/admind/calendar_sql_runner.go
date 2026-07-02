package admind

import (
	"context"
	"database/sql"
)

type calendarSQLRunner interface {
	ExecContext(ctx context.Context, query string, arguments ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, arguments ...any) (*sql.Rows, error)
}
