import { addTaskWeekDays, taskWeekDateFromISO, taskWeekDateISO } from '$lib/task/task-week-code';
import { BOARD_STATUS_VALUES, type TaskBoardStatus } from './task-board-model';
import type { TaskChildProgress } from './task-relationships';
import { isTaskStatusCompleted } from './task-status';
import type { Task } from './task-types';

export type TaskRoadmapMonth = {
	year: number;
	month: number;
};

export type TaskRoadmapMonthSegment = {
	month: TaskRoadmapMonth;
	firstDay: number;
	dayCount: number;
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

export const taskRoadmapMonthRadius = 6;

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

export function formatTaskRoadmapWeekday(dateISO: string, locale: string): string {
	return new Intl.DateTimeFormat(locale, { weekday: 'narrow', timeZone: 'UTC' }).format(new Date(`${dateISO}T00:00:00Z`));
}

export function isTaskRoadmapWeekend(dateISO: string): boolean {
	const weekday = taskWeekDateFromISO(dateISO)?.getUTCDay();
	return weekday === 0 || weekday === 6;
}

export function taskRoadmapDays(centerMonth: TaskRoadmapMonth, monthRadius = taskRoadmapMonthRadius): string[] {
	const first = new Date(Date.UTC(centerMonth.year, centerMonth.month - 1 - monthRadius, 1));
	const afterLast = new Date(Date.UTC(centerMonth.year, centerMonth.month + monthRadius, 1));
	const dayCount = Math.round((afterLast.getTime() - first.getTime()) / 86_400_000);
	return Array.from({ length: dayCount }, (_, index) => taskWeekDateISO(addTaskWeekDays(first, index)));
}

export function taskRoadmapMonthSegments(days: string[]): TaskRoadmapMonthSegment[] {
	return days.reduce<TaskRoadmapMonthSegment[]>((segments, dayISO, index) => {
		const month = taskRoadmapMonthOf(dayISO);
		const last = segments.at(-1);
		if (last && taskRoadmapMonthKey(last.month) === taskRoadmapMonthKey(month)) {
			return [...segments.slice(0, -1), { ...last, dayCount: last.dayCount + 1 }];
		}
		return [...segments, { month, firstDay: index, dayCount: 1 }];
	}, []);
}

export function buildTaskRoadmap(
	tasks: Task[],
	days: string[],
	childProgressByParent: ReadonlyMap<string, TaskChildProgress>
): TaskRoadmapGroup[] {
	return BOARD_STATUS_VALUES.map((status) => ({
		status,
		rows: tasks
			.filter((task) => task.status === status)
			.map((task) => ({ task, bar: taskRoadmapBar(task, days), childProgress: childProgressByParent.get(task.id) }))
			.filter((row) => isShownInRange(row))
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

export function moveTaskDateRange(range: TaskDateRange, dayOffset: number): TaskDateRange {
	return { startDate: shiftDateISO(range.startDate, dayOffset), endDate: shiftDateISO(range.endDate, dayOffset) };
}

function shiftDateISO(dateISO: string, offset: number): string {
	const date = taskWeekDateFromISO(dateISO);
	return date ? taskWeekDateISO(addTaskWeekDays(date, offset)) : dateISO;
}

function validDateISO(value: string | undefined): string | undefined {
	const trimmed = value?.trim() ?? '';
	return taskWeekDateFromISO(trimmed) ? trimmed : undefined;
}

function isShownInRange(row: TaskRoadmapRow): boolean {
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
