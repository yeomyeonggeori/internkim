import { companyTimeInstant } from './supabase-work-status-range';
import { requiredClockMinute } from './current-work-policy';
import { scheduleEndMinute } from './work-schedule-window';
import type { AttendanceWorkPolicyRevision } from './work-calendar-derivation';

export type DayMinuteInterval = { startMinute: number; endMinute: number };

export type LeaveDayRow = { days: number; starts_at: string; ends_at: string };

const minutesPerDay = 24 * 60;

export function leaveDayIntervals(
	rows: LeaveDayRow[],
	day: string,
	timeZone: string,
	revision: AttendanceWorkPolicyRevision
): DayMinuteInterval[] {
	const dayStart = new Date(companyTimeInstant(day, '00:00', timeZone)).getTime();
	const intervals: DayMinuteInterval[] = [];
	for (const row of rows) {
		const interval =
			row.days > 0.5
				? scheduledDayInterval(revision)
				: rowSpanInterval(row, dayStart, timeZone, day);
		if (interval && interval.endMinute > interval.startMinute) intervals.push(interval);
	}
	return intervals;
}

export function scheduledDayInterval(
	revision: AttendanceWorkPolicyRevision
): DayMinuteInterval | undefined {
	if (revision.dailyTargetMinutes <= 0) return undefined;
	const startTime =
		revision.workMode === 'fixed' ? revision.fixedStartTime : revision.referenceStartTime;
	if (!startTime) return undefined;
	const startMinute = requiredClockMinute(startTime, 'scheduleStartTime');
	return {
		startMinute,
		endMinute: scheduleEndMinute(startMinute, revision.dailyTargetMinutes, revision.breakPeriods)
	};
}

function rowSpanInterval(
	row: LeaveDayRow,
	dayStart: number,
	timeZone: string,
	day: string
): DayMinuteInterval {
	const dayEnd = new Date(companyTimeInstant(day, '00:00', timeZone)).getTime() + minutesPerDay * 60_000;
	const startMinute = clampedMinute(new Date(row.starts_at).getTime(), dayStart, dayEnd);
	const endMinute = clampedMinute(new Date(row.ends_at).getTime(), dayStart, dayEnd);
	return { startMinute, endMinute };
}

function clampedMinute(instant: number, dayStart: number, dayEnd: number): number {
	const bounded = Math.min(Math.max(instant, dayStart), dayEnd);
	return Math.round((bounded - dayStart) / 60_000);
}
