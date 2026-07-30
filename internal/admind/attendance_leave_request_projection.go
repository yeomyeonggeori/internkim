package admind

import (
	"context"
	"database/sql"
	"strings"
)

func linkAttendanceLeaveRequestAbsenceRange(
	ctx context.Context,
	transaction *sql.Tx,
	requestID string,
	rangeID string,
) error {
	_, errorValue := transaction.ExecContext(ctx, `
INSERT OR IGNORE INTO attendance_leave_request_absence_ranges (request_id, range_id)
VALUES (?, ?)`,
		strings.TrimSpace(requestID),
		strings.TrimSpace(rangeID),
	)
	return errorValue
}
