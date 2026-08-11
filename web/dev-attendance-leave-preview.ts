import { isWeekday } from './src/routes/attendance/shared/attendance-date';
import type {
	EmployeeLeavePreview,
	EmployeeLeavePreviewRequest
} from './src/routes/attendance/leave/employee-leave-types';
import { leavePreviewPeriod } from './src/lib/attendance/supabase-leave-range';

const fixtureHolidayDates = new Set(['2026-08-17']);

export function buildEmployeeLeavePreview(
	request: EmployeeLeavePreviewRequest
): EmployeeLeavePreview {
	const dates =
		request.unit === 'fullDay'
			? datesBetween(request.startDate, request.endDate || request.startDate)
			: [request.startDate];
	const occurrences: EmployeeLeavePreview['occurrences'] = [];
	const excludedDates: EmployeeLeavePreview['excludedDates'] = [];
	for (const date of dates) {
		if (!isWeekday(date)) {
			excludedDates.push({ date, reason: 'nonWorkingDay' });
			continue;
		}
		if (fixtureHolidayDates.has(date)) {
			excludedDates.push({ date, reason: 'holiday' });
			continue;
		}
		const period = leavePreviewPeriod(request);
		occurrences.push({
			date,
			startTime: period.startTime,
			endTime: period.endTime,
			deductionMilliDays: period.deductionMilliDays
		});
	}
	return {
		occurrences,
		excludedDates,
		totalDeductionMilliDays: occurrences.reduce(
			(total, occurrence) => total + occurrence.deductionMilliDays,
			0
		)
	};
}

function datesBetween(startDate: string, endDate: string): string[] {
	const start = new Date(`${startDate}T00:00:00Z`);
	const end = new Date(`${endDate}T00:00:00Z`);
	if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || end < start) {
		return [startDate];
	}
	const dates: string[] = [];
	const cursor = new Date(start);
	while (cursor <= end && dates.length < 62) {
		dates.push(cursor.toISOString().slice(0, 10));
		cursor.setUTCDate(cursor.getUTCDate() + 1);
	}
	return dates;
}
