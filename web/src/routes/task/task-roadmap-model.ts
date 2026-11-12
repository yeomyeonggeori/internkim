import { addTaskWeekDays, taskWeekDateFromISO, taskWeekDateISO } from '$lib/task/task-week-code';
import type { TaskChildProgress } from './task-relationships';
import type { Task } from './task-types';

export type TaskRoadmapMonth = {
	year: number;
	month: number;
};

export type TaskRoadmapBar =
	| { kind: 'span'; startDate: string; endDate: string; firstDay: number; lastDay: number; startsBefore: boolean; endsAfter: boolean }
	| { kind: 'before'; startDate: string; endDate: string }
	| { kind: 'after'; startDate: string; endDate: string }
	| { kind: 'undated' };

export type TaskRoadmapRow = {
	task: Task;
	bar: TaskRoadmapBar;
	childProgress: TaskChildProgress | undefined;
};

export type TaskDateRange = {
	startDate: string;
	endDate: string;
};

export const taskRoadmapMonthRadius = 6;
const millisecondsPerDay = 86_400_000;

export function localDateISO(date: Date): string {
	return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

export function taskRoadmapMonthOf(dateISO: string): TaskRoadmapMonth {
	const date = taskWeekDateFromISO(dateISO) ?? new Date();
	return { year: date.getUTCFullYear(), month: date.getUTCMonth() + 1 };
}

export function shiftTaskRoadmapMonth(month: TaskRoadmapMonth, offset: number): TaskRoadmapMonth {
	const shifted = new Date(Date.UTC(month.year, month.month - 1 + offset, 1));
	return { year: shifted.getUTCFullYear(), month: shifted.getUTCMonth() + 1 };
}

export function taskRoadmapMonthKey(month: TaskRoadmapMonth): string {
	return `${month.year}-${String(month.month).padStart(2, '0')}`;
}

export function formatTaskRoadmapWeekday(dateISO: string, locale: string): string {
	return new Intl.DateTimeFormat(locale, { weekday: 'narrow', timeZone: 'UTC' }).format(new Date(`${dateISO}T00:00:00Z`));
}

export function isTaskRoadmapWeekend(dateISO: string): boolean {
	const weekday = taskWeekDateFromISO(dateISO)?.getUTCDay();
	return weekday === 0 || weekday === 6;
}

export function taskRoadmapDays(startISO: string, dayCount: number): string[] {
	return Array.from({ length: dayCount }, (_, index) => shiftTaskRoadmapDate(startISO, index));
}

export function buildTaskRoadmapRows(
	tasks: Task[],
	days: string[],
	childProgressByParent: ReadonlyMap<string, TaskChildProgress>
): TaskRoadmapRow[] {
	return tasks
		.map((task) => ({ task, bar: taskRoadmapBar(task, days), childProgress: childProgressByParent.get(task.id) }))
		.toSorted(compareTaskRoadmapRows);
}

export function taskRoadmapBar(task: Task, days: string[]): TaskRoadmapBar {
	const range = taskDateRange(task);
	const firstDayISO = days[0];
	const lastDayISO = days.at(-1);
	if (!range || !firstDayISO || !lastDayISO) return { kind: 'undated' };
	if (range.endDate < firstDayISO) return { kind: 'before', ...range };
	if (range.startDate > lastDayISO) return { kind: 'after', ...range };
	const startsBefore = range.startDate < firstDayISO;
	const endsAfter = range.endDate > lastDayISO;
	return {
		kind: 'span',
		...range,
		firstDay: startsBefore ? 0 : days.indexOf(range.startDate),
		lastDay: endsAfter ? days.length - 1 : days.indexOf(range.endDate),
		startsBefore,
		endsAfter
	};
}

export function taskDateRange(task: Pick<Task, 'startDate' | 'endDate'>): TaskDateRange | undefined {
	const startDate = validDateISO(task.startDate) ?? validDateISO(task.endDate);
	const endDate = validDateISO(task.endDate) ?? startDate;
	if (!startDate || !endDate) return undefined;
	return startDate <= endDate ? { startDate, endDate } : { startDate: endDate, endDate: startDate };
}

export function moveTaskDateRange(range: TaskDateRange, dayOffset: number): TaskDateRange {
	return { startDate: shiftTaskRoadmapDate(range.startDate, dayOffset), endDate: shiftTaskRoadmapDate(range.endDate, dayOffset) };
}

export function clampTaskRoadmapMove(range: TaskDateRange, days: string[], dayOffset: number): number {
	const firstDayISO = days[0];
	const lastDayISO = days.at(-1);
	if (!firstDayISO || !lastDayISO) return dayOffset;
	const earliest = taskRoadmapDaysBetween(range.endDate, firstDayISO);
	const latest = taskRoadmapDaysBetween(range.startDate, lastDayISO);
	return Math.min(Math.max(dayOffset, earliest), latest);
}

export function shiftTaskRoadmapDate(dateISO: string, offset: number): string {
	const date = taskWeekDateFromISO(dateISO);
	return date ? taskWeekDateISO(addTaskWeekDays(date, offset)) : dateISO;
}

export function taskRoadmapDaysBetween(fromISO: string, toISO: string): number {
	const from = taskWeekDateFromISO(fromISO);
	const to = taskWeekDateFromISO(toISO);
	if (!from || !to) return 0;
	return Math.round((to.getTime() - from.getTime()) / millisecondsPerDay);
}

export function formatTaskRoadmapShortDate(dateISO: string): string {
	return `${Number(dateISO.slice(5, 7))}/${Number(dateISO.slice(8, 10))}`;
}

export function formatTaskRoadmapDayNumber(dateISO: string): string {
	const day = Number(dateISO.slice(8, 10));
	return day === 1 ? formatTaskRoadmapShortDate(dateISO) : String(day);
}

export function formatTaskRoadmapPeriod(range: TaskDateRange): string {
	const start = formatTaskRoadmapShortDate(range.startDate);
	if (range.startDate === range.endDate) return start;
	return `${start} – ${formatTaskRoadmapShortDate(range.endDate)}`;
}

function validDateISO(value: string | undefined): string | undefined {
	const trimmed = value?.trim() ?? '';
	return taskWeekDateFromISO(trimmed) ? trimmed : undefined;
}

function compareTaskRoadmapRows(left: TaskRoadmapRow, right: TaskRoadmapRow): number {
	const startComparison = compareOptionalDate(taskDateRange(left.task)?.startDate, taskDateRange(right.task)?.startDate);
	if (startComparison !== 0) return startComparison;
	const createdComparison = compareOptionalDate(left.task.createdAt, right.task.createdAt);
	if (createdComparison !== 0) return createdComparison;
	return left.task.id.localeCompare(right.task.id);
}

function compareOptionalDate(left: string | undefined, right: string | undefined): number {
	if (left === right) return 0;
	if (!left) return 1;
	if (!right) return -1;
	return left.localeCompare(right);
}
