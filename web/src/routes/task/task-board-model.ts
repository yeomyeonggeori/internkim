import type { Task } from './task-types';

export const BOARD_STATUS_VALUES = ['요청', '예정', '진행', '완료', '일시정지'] as const;

export type TaskBoardStatus = (typeof BOARD_STATUS_VALUES)[number];

export type TaskBoardColumn = {
	status: TaskBoardStatus;
	theme: TaskBoardColumnTheme;
	tasks: Task[];
};

export type TaskBoardColumnTheme = {
	accentColor: string;
	dotClass: string;
	titleClass: string;
	headerClass: string;
};

export type TaskBoardOptions = {
	weekStartISO?: string;
	weekEndISO?: string;
	weekPosition?: TaskBoardWeekPosition;
	hideEmptyRequestColumn?: boolean;
};

export type TaskBoardWeekPosition = 'past' | 'current' | 'future';

const boardColumnThemes: Record<TaskBoardStatus, TaskBoardColumnTheme> = {
	요청: {
		accentColor: '#7c3aed',
		dotClass: 'bg-[#7c3aed]',
		titleClass: 'text-[#4c1d95]',
		headerClass: 'bg-[#f3e8ff]/70'
	},
	예정: {
		accentColor: '#d97706',
		dotClass: 'bg-[#d97706]',
		titleClass: 'text-[#78350f]',
		headerClass: 'bg-[#fef3c7]/80'
	},
	진행: {
		accentColor: '#0284c7',
		dotClass: 'bg-[#0284c7]',
		titleClass: 'text-[#075985]',
		headerClass: 'bg-[#e0f2fe]/80'
	},
	완료: {
		accentColor: '#16a34a',
		dotClass: 'bg-[#16a34a]',
		titleClass: 'text-[#166534]',
		headerClass: 'bg-[#dcfce7]/80'
	},
	일시정지: {
		accentColor: '#e11d48',
		dotClass: 'bg-[#e11d48]',
		titleClass: 'text-[#9f1239]',
		headerClass: 'bg-[#ffe4e6]/80'
	}
};

export function buildTaskBoard(tasks: Task[], options: TaskBoardOptions = {}): TaskBoardColumn[] {
	return BOARD_STATUS_VALUES.filter((status) => isBoardColumnVisible(status, options))
		.map((status) => ({
			status,
			theme: boardColumnThemes[status],
			tasks: tasks
				.filter((task) => task.status === status && matchesBoardColumnWeek(task, status, options))
				.toSorted(compareTaskBoardOrder)
		}))
		.filter((column) => !(column.status === '요청' && column.tasks.length === 0 && options.hideEmptyRequestColumn));
}

export function isTaskBoardStatus(status: string): status is TaskBoardStatus {
	return BOARD_STATUS_VALUES.some((boardStatus) => boardStatus === status);
}

function compareTaskBoardOrder(left: Task, right: Task): number {
	const rankDifference = left.statusRank - right.statusRank;
	if (rankDifference !== 0) return rankDifference;
	return left.id.localeCompare(right.id);
}

export function isOverdueTaskPlan(task: Task, weekStartISO: string): boolean {
	if (task.status !== '예정' || !weekStartISO) return false;
	const plannedDate = task.endDate?.trim() || task.startDate?.trim() || '';
	return plannedDate !== '' && plannedDate < weekStartISO;
}

function isBoardColumnVisible(status: TaskBoardStatus, options: TaskBoardOptions): boolean {
	return status !== '예정' || weekPosition(options) !== 'past';
}

function matchesBoardColumnWeek(task: Task, status: TaskBoardStatus, options: TaskBoardOptions): boolean {
	if (status === '요청' || status === '일시정지') return true;
	if (status === '진행') return weekPosition(options) === 'current';
	if (!options.weekStartISO || !options.weekEndISO) return true;
	if (status === '완료') return isInSelectedWeek(task.endDate, options);
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
