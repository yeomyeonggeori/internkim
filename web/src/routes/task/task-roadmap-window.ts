import { taskWeekDateFromISO } from '$lib/task/task-week-code';
import {
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

export function taskRoadmapMonthStart(month: TaskRoadmapMonth, todayISO: string): string {
	const monthKey = taskRoadmapMonthKey(month);
	if (monthKey === taskRoadmapMonthKey(taskRoadmapMonthOf(todayISO))) return taskRoadmapWeekStart(todayISO);
	const firstWeekStart = taskRoadmapWeekStart(`${monthKey}-01`);
	if (taskRoadmapMonthKey(taskRoadmapShownMonth(firstWeekStart)) === monthKey) return firstWeekStart;
	return shiftTaskRoadmapDate(firstWeekStart, taskRoadmapWeekLength);
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
