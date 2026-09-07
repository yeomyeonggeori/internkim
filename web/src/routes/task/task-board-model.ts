import type { Task } from './task-types';

export const BOARD_STATUS_VALUES = ['requested', 'planned', 'in_progress', 'completed', 'paused'] as const;

export type TaskBoardStatus = (typeof BOARD_STATUS_VALUES)[number];

export type TaskBoardColumn = {
	status: TaskBoardStatus;
	tasks: Task[];
};

export type TaskBoardOptions = {
	weekStartISO?: string;
	weekEndISO?: string;
	weekPosition?: TaskBoardWeekPosition;
	hideEmptyRequestColumn?: boolean;
};

export type TaskBoardWeekPosition = 'past' | 'current' | 'future';

export function buildTaskBoard(tasks: Task[], options: TaskBoardOptions = {}): TaskBoardColumn[] {
	return BOARD_STATUS_VALUES.filter((status) => isBoardColumnVisible(status, options))
		.map((status) => ({
			status,
			tasks: tasks
				.filter((task) => task.status === status && matchesBoardColumnWeek(task, status, options))
				.toSorted(compareTaskBoardOrder)
		}))
		.filter((column) => !(column.status === 'requested' && column.tasks.length === 0 && options.hideEmptyRequestColumn));
}

export function isTaskBoardStatus(status: string): status is TaskBoardStatus {
	return BOARD_STATUS_VALUES.some((boardStatus) => boardStatus === status);
}

function compareTaskBoardOrder(left: Task, right: Task): number {
	const endDateComparison = compareAscendingOptionalDate(left.endDate, right.endDate);
	if (endDateComparison !== 0) return endDateComparison;
	const createdAtComparison = compareAscendingOptionalDate(left.createdAt, right.createdAt);
	if (createdAtComparison !== 0) return createdAtComparison;
	return left.id.localeCompare(right.id);
}

function compareAscendingOptionalDate(left: string | undefined, right: string | undefined): number {
	const leftValue = left?.trim() ?? '';
	const rightValue = right?.trim() ?? '';
	if (leftValue === rightValue) return 0;
	if (leftValue === '') return 1;
	if (rightValue === '') return -1;
	return leftValue.localeCompare(rightValue);
}

export function isOverdueTaskPlan(task: Task, weekStartISO: string): boolean {
	if (task.status !== 'planned' || !weekStartISO) return false;
	const plannedDate = task.endDate?.trim() || task.startDate?.trim() || '';
	return plannedDate !== '' && plannedDate < weekStartISO;
}

function isBoardColumnVisible(status: TaskBoardStatus, options: TaskBoardOptions): boolean {
	return status !== 'planned' || weekPosition(options) !== 'past';
}

function matchesBoardColumnWeek(task: Task, status: TaskBoardStatus, options: TaskBoardOptions): boolean {
	if (status === 'requested' || status === 'paused') return true;
	if (status === 'in_progress') return weekPosition(options) === 'current';
	if (!options.weekStartISO || !options.weekEndISO) return true;
	if (status === 'completed') return isInSelectedWeek(task.endDate, options);
	const plannedDate = task.endDate?.trim() || task.startDate?.trim() || '';
	if (!plannedDate) return weekPosition(options) === 'current';
	if (plannedDate < options.weekStartISO) return weekPosition(options) === 'current';
	return isInSelectedWeek(plannedDate, options);
}

function weekPosition(options: TaskBoardOptions): TaskBoardWeekPosition {
	return options.weekPosition ?? 'current';
}

function isInSelectedWeek(date: string | undefined, options: TaskBoardOptions): boolean {
	const selectedDate = date?.trim() ?? '';
	if (!selectedDate || !options.weekStartISO || !options.weekEndISO) return false;
	return selectedDate >= options.weekStartISO && selectedDate <= options.weekEndISO;
}
