package admind

import (
	"context"
	"database/sql"
)

func invalidateAttendanceAbsenceCacheDates(
	ctx context.Context,
	transaction *sql.Tx,
	dates []string,
) error {
	if len(dates) == 0 {
		return nil
	}
	months, errorValue := attendanceAbsenceCacheMonthsForDates(dates)
	if errorValue != nil {
		return errorValue
	}
	return invalidateAttendanceSummaryCacheMonths(ctx, transaction, attendanceSummaryCacheKindAbsences, months)
}
