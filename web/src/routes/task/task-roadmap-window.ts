import { taskWeekDateFromISO } from '$lib/task/task-week-code';
import {
	formatTaskRoadmapPeriod,
	shiftTaskRoadmapDate,
	shiftTaskRoadmapMonth,
	taskRoadmapMonthKey,
	taskRoadmapMonthOf,
	taskRoadmapMonthRadius,
	type TaskRoadmapMonth
} from './task-roadmap-model';

export const taskRoadmapWideDayCount = 14;
export const taskRoadmapNarrowDayCount = 7;
export const taskRoadmapWeekLength = 7;
const shownMonthDayOffset = 3;

export type TaskRoadmapWindowSpan = {
	firstDayISO: string;
	lastDayISO: string;
};

export function taskRoadmapWeekStart(dateISO: string): string {
	const weekday = taskWeekDateFromISO(dateISO)?.getUTCDay() ?? 0;
	return shiftTaskRoadmapDate(dateISO, -weekday);
}

export function taskRoadmapShownMonth(startISO: string): TaskRoadmapMonth {
	return taskRoadmapMonthOf(shiftTaskRoadmapDate(startISO, shownMonthDayOffset));
}

export function taskRoadmapDayCount(isNarrow: boolean): number {
	return isNarrow ? taskRoadmapNarrowDayCount : taskRoadmapWideDayCount;
}

export function taskRoadmapWeekChoices(todayISO: string): string[] {
	const currentWeekStart = taskRoadmapWeekStart(todayISO);
	const earlier = allowedWeekStartsFrom(currentWeekStart, -taskRoadmapWeekLength, todayISO);
	const later = allowedWeekStartsFrom(currentWeekStart, taskRoadmapWeekLength, todayISO);
	return [...earlier.toReversed(), currentWeekStart, ...later];
}

export function formatTaskRoadmapWindow(startISO: string, dayCount: number, todayISO: string): string {
	const period = formatTaskRoadmapPeriod({ startDate: startISO, endDate: shiftTaskRoadmapDate(startISO, dayCount - 1) });
	const year = startISO.slice(0, 4);
	return year === todayISO.slice(0, 4) ? period : `${year} ${period}`;
}

export function isTaskRoadmapStartAllowed(startISO: string, todayISO: string): boolean {
	const shown = taskRoadmapShownMonth(startISO);
	const current = taskRoadmapMonthOf(todayISO);
	const monthOffset = (shown.year - current.year) * 12 + shown.month - current.month;
	return Math.abs(monthOffset) <= taskRoadmapMonthRadius;
}

export function taskRoadmapWindowSpan(todayISO: string): TaskRoadmapWindowSpan {
	const current = taskRoadmapMonthOf(todayISO);
	const firstMonth = shiftTaskRoadmapMonth(current, -taskRoadmapMonthRadius);
	const afterLastMonth = shiftTaskRoadmapMonth(current, taskRoadmapMonthRadius + 1);
	return {
		firstDayISO: shiftTaskRoadmapDate(`${taskRoadmapMonthKey(firstMonth)}-01`, -taskRoadmapWeekLength),
		lastDayISO: shiftTaskRoadmapDate(`${taskRoadmapMonthKey(afterLastMonth)}-01`, taskRoadmapWideDayCount)
	};
}

function allowedWeekStartsFrom(weekStartISO: string, step: number, todayISO: string): string[] {
	const weekStarts: string[] = [];
	let candidate = shiftTaskRoadmapDate(weekStartISO, step);
	while (isTaskRoadmapStartAllowed(candidate, todayISO)) {
		weekStarts.push(candidate);
		candidate = shiftTaskRoadmapDate(candidate, step);
	}
	return weekStarts;
}
