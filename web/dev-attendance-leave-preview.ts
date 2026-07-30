import { isWeekday } from './src/routes/attendance/shared/attendance-date';
import type {
	EmployeeLeavePreview,
	EmployeeLeavePreviewRequest
} from './src/routes/attendance/leave/employee-leave-types';

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
		const period = previewPeriod(request);
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

function previewPeriod(request: EmployeeLeavePreviewRequest): {
	startTime: string;
	endTime: string;
	deductionMilliDays: number;
} {
	if (request.unit === 'fullDay') {
		return { startTime: '09:00', endTime: '18:00', deductionMilliDays: 1000 };
	}
	if (request.partialPeriod === 'afternoon') {
		return {
			startTime: request.unit === 'halfDay' ? '14:00' : '16:00',
			endTime: '18:00',
			deductionMilliDays: request.unit === 'halfDay' ? 500 : 250
		};
	}
	if (request.partialPeriod === 'custom') {
		const startTime = request.startTime || '09:00';
		return {
			startTime,
			endTime: addWorkMinutes(startTime, request.unit === 'halfDay' ? 240 : 120),
			deductionMilliDays: request.unit === 'halfDay' ? 500 : 250
		};
	}
	return {
		startTime: '09:00',
		endTime: request.unit === 'halfDay' ? '14:00' : '11:00',
		deductionMilliDays: request.unit === 'halfDay' ? 500 : 250
	};
}

function addWorkMinutes(startTime: string, workMinutes: number): string {
	const [hour = 9, minute = 0] = startTime.split(':').map(Number);
	let totalMinutes = hour * 60 + minute + workMinutes;
	const lunchStartMinutes = 12 * 60;
	if (hour * 60 + minute < lunchStartMinutes && totalMinutes > lunchStartMinutes) {
		totalMinutes += 60;
	}
	const normalizedMinutes = Math.min(totalMinutes, 23 * 60 + 59);
	return `${String(Math.floor(normalizedMinutes / 60)).padStart(2, '0')}:${String(normalizedMinutes % 60).padStart(2, '0')}`;
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
