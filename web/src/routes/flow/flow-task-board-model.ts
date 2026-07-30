import type { FlowTask } from './flow-types';

export const BOARD_STATUS_VALUES = ['요청', '예정', '진행', '완료', '일시정지'] as const;

export type FlowTaskBoardStatus = (typeof BOARD_STATUS_VALUES)[number];

export type FlowTaskBoardColumn = {
	status: FlowTaskBoardStatus;
	theme: FlowTaskBoardColumnTheme;
	tasks: FlowTask[];
};

export type FlowTaskBoardColumnTheme = {
	accentColor: string;
	dotClass: string;
	titleClass: string;
	headerClass: string;
};

export type FlowTaskBoardOptions = {
	weekStartISO?: string;
	weekEndISO?: string;
	weekPosition?: FlowTaskBoardWeekPosition;
};

export type FlowTaskBoardWeekPosition = 'past' | 'current' | 'future';

const boardColumnThemes: Record<FlowTaskBoardStatus, FlowTaskBoardColumnTheme> = {
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

export function buildFlowTaskBoard(tasks: FlowTask[], options: FlowTaskBoardOptions = {}): FlowTaskBoardColumn[] {
	return BOARD_STATUS_VALUES.filter((status) => isBoardColumnVisible(status, options)).map((status) => ({
		status,
		theme: boardColumnThemes[status],
		tasks: tasks
			.filter((task) => task.status === status && matchesBoardColumnWeek(task, status, options))
			.toSorted(compareFlowTaskBoardOrder)
	}));
}

export function isFlowTaskBoardStatus(status: string): status is FlowTaskBoardStatus {
	return BOARD_STATUS_VALUES.some((boardStatus) => boardStatus === status);
}

function compareFlowTaskBoardOrder(left: FlowTask, right: FlowTask): number {
	const rankDifference = left.statusRank - right.statusRank;
	if (rankDifference !== 0) return rankDifference;
	return left.id.localeCompare(right.id);
}

function isBoardColumnVisible(status: FlowTaskBoardStatus, options: FlowTaskBoardOptions): boolean {
	return status !== '예정' || weekPosition(options) !== 'past';
}

function matchesBoardColumnWeek(task: FlowTask, status: FlowTaskBoardStatus, options: FlowTaskBoardOptions): boolean {
	if (status === '요청' || status === '일시정지') return true;
	if (status === '진행') return weekPosition(options) === 'current';
	if (!options.weekStartISO || !options.weekEndISO) return true;
	if (status === '완료') return isInSelectedWeek(task.endDate, options);
	const plannedDate = task.endDate?.trim() || task.startDate?.trim() || '';
	if (!plannedDate) return weekPosition(options) === 'current';
	if (plannedDate < options.weekStartISO) return weekPosition(options) === 'current';
	return isInSelectedWeek(plannedDate, options);
}

function weekPosition(options: FlowTaskBoardOptions): FlowTaskBoardWeekPosition {
	return options.weekPosition ?? 'current';
}

function isInSelectedWeek(date: string | undefined, options: FlowTaskBoardOptions): boolean {
	const selectedDate = date?.trim() ?? '';
	if (!selectedDate || !options.weekStartISO || !options.weekEndISO) return false;
	return selectedDate >= options.weekStartISO && selectedDate <= options.weekEndISO;
}
