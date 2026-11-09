import { addTaskWeekDays, taskWeekDateFromISO, taskWeekDateISO } from '$lib/task/task-week-code';
import { BOARD_STATUS_VALUES, type TaskBoardStatus } from './task-board-model';
import type { TaskChildProgress } from './task-relationships';
import { isTaskStatusCompleted } from './task-status';
import type { Task } from './task-types';

export type TaskRoadmapMonth = {
	year: number;
	month: number;
};

export type TaskRoadmapBar =
	| { kind: 'span'; firstDay: number; lastDay: number; startsBefore: boolean; endsAfter: boolean }
	| { kind: 'before'; dateISO: string }
	| { kind: 'after'; dateISO: string }
	| { kind: 'undated' };

export type TaskRoadmapRow = {
	task: Task;
	bar: TaskRoadmapBar;
	childProgress: TaskChildProgress | undefined;
};

export type TaskRoadmapGroup = {
	status: TaskBoardStatus;
	rows: TaskRoadmapRow[];
};

export type TaskDateRange = {
	startDate: string;
	endDate: string;
};

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

export function taskRoadmapMonthFromKey(key: string): TaskRoadmapMonth | undefined {
	const date = taskWeekDateFromISO(`${key}-01`);
	return date ? { year: date.getUTCFullYear(), month: date.getUTCMonth() + 1 } : undefined;
}

export function formatTaskRoadmapMonth(month: TaskRoadmapMonth, locale: string): string {
	return new Intl.DateTimeFormat(locale, { year: 'numeric', month: 'long', timeZone: 'UTC' })
		.format(new Date(Date.UTC(month.year, month.month - 1, 1)));
}

export function taskRoadmapMonthDays(month: TaskRoadmapMonth): string[] {
	const first = new Date(Date.UTC(month.year, month.month - 1, 1));
	const dayCount = new Date(Date.UTC(month.year, month.month, 0)).getUTCDate();
	return Array.from({ length: dayCount }, (_, index) => taskWeekDateISO(addTaskWeekDays(first, index)));
}

export function buildTaskRoadmap(
	tasks: Task[],
	month: TaskRoadmapMonth,
	childProgressByParent: ReadonlyMap<string, TaskChildProgress>
): TaskRoadmapGroup[] {
	const days = taskRoadmapMonthDays(month);
	return BOARD_STATUS_VALUES.map((status) => ({
		status,
		rows: tasks
			.filter((task) => task.status === status)
			.map((task) => ({ task, bar: taskRoadmapBar(task, days), childProgress: childProgressByParent.get(task.id) }))
			.filter((row) => isShownInMonth(row))
			.toSorted(compareTaskRoadmapRows)
	})).filter((group) => group.rows.length > 0);
}

export function taskRoadmapBar(task: Task, days: string[]): TaskRoadmapBar {
	const range = taskDateRange(task);
	const firstDayISO = days[0];
	const lastDayISO = days.at(-1);
	if (!range || !firstDayISO || !lastDayISO) return { kind: 'undated' };
	if (range.endDate < firstDayISO) return { kind: 'before', dateISO: range.endDate };
	if (range.startDate > lastDayISO) return { kind: 'after', dateISO: range.startDate };
	const startsBefore = range.startDate < firstDayISO;
	const endsAfter = range.endDate > lastDayISO;
	return {
		kind: 'span',
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

export function shiftTaskDateRange(range: TaskDateRange, startOffset: number, endOffset: number): TaskDateRange {
	const startDate = shiftDateISO(range.startDate, startOffset);
	const endDate = shiftDateISO(range.endDate, endOffset);
	if (startDate <= endDate) return { startDate, endDate };
	return startOffset === 0 ? { startDate, endDate: startDate } : { startDate: endDate, endDate };
}

function shiftDateISO(dateISO: string, offset: number): string {
	const date = taskWeekDateFromISO(dateISO);
	return date ? taskWeekDateISO(addTaskWeekDays(date, offset)) : dateISO;
}

function validDateISO(value: string | undefined): string | undefined {
	const trimmed = value?.trim() ?? '';
	return taskWeekDateFromISO(trimmed) ? trimmed : undefined;
}

function isShownInMonth(row: TaskRoadmapRow): boolean {
	if (row.bar.kind === 'span') return true;
	return !isTaskStatusCompleted(row.task.status);
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
