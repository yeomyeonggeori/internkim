import { describe, expect, test } from 'bun:test';
import { BOARD_STATUS_VALUES, buildFlowTaskBoard } from '../../src/routes/flow/flow-task-board-model';
import type { FlowTask } from '../../src/routes/flow/flow-types';

describe('flow task board model', () => {
	test('groups only board statuses and excludes rejected or stopped tasks', () => {
		const board = buildFlowTaskBoard([
			flowTask({ id: 'requested-1', status: '요청', statusRank: 100 }),
			flowTask({ id: 'rejected-1', status: '기각', statusRank: 100 }),
			flowTask({ id: 'stopped-1', status: '중단', statusRank: 100 }),
			flowTask({ id: 'paused-1', status: '일시정지', statusRank: 100 })
		]);

		expect(board.map((column) => column.status)).toEqual(BOARD_STATUS_VALUES);
		expect(board.flatMap((column) => column.tasks.map((task) => task.id))).toEqual(['requested-1', 'paused-1']);
	});

	test('sorts tasks from top to bottom by status rank inside each column', () => {
		const board = buildFlowTaskBoard([
			flowTask({ id: 'bottom', status: '진행', statusRank: 300 }),
			flowTask({ id: 'top', status: '진행', statusRank: 100 }),
			flowTask({ id: 'middle', status: '진행', statusRank: 200 })
		]);

		const progressColumn = board.find((column) => column.status === '진행');

		expect(progressColumn?.tasks.map((task) => task.id)).toEqual(['top', 'middle', 'bottom']);
	});

	test('uses task id as a stable tie-break when status ranks match', () => {
		const board = buildFlowTaskBoard([
			flowTask({ id: 'task-c', status: '진행', statusRank: 100 }),
			flowTask({ id: 'task-a', status: '진행', statusRank: 100 }),
			flowTask({ id: 'task-b', status: '진행', statusRank: 100 })
		]);

		const progressColumn = board.find((column) => column.status === '진행');

		expect(progressColumn?.tasks.map((task) => task.id)).toEqual(['task-a', 'task-b', 'task-c']);
	});

	test('assigns a visible accent theme to every board column status', () => {
		const board = buildFlowTaskBoard([]);

		expect(board.map((column) => [column.status, column.theme.dotClass])).toEqual([
			['요청', 'bg-[#7c3aed]'],
			['예정', 'bg-[#d97706]'],
			['진행', 'bg-[#0284c7]'],
			['완료', 'bg-[#16a34a]'],
			['일시정지', 'bg-[#e11d48]']
		]);
	});

	test('keeps planned tasks in the week of their end date, or their start date when open ended', () => {
		const board = buildFlowTaskBoard([
			flowTask({ id: 'ends-this-week', status: '예정', startDate: '2026-05-25', endDate: '2026-06-03' }),
			flowTask({ id: 'ends-next-week', status: '예정', startDate: '2026-06-02', endDate: '2026-06-10' }),
			flowTask({ id: 'starts-this-week-open-ended', status: '예정', startDate: '2026-06-04', endDate: '' })
		], {
			weekStartISO: '2026-06-01',
			weekEndISO: '2026-06-07',
			weekPosition: 'future'
		});

		expect(board.find((column) => column.status === '예정')?.tasks.map((task) => task.id)).toEqual([
			'ends-this-week',
			'starts-this-week-open-ended'
		]);
	});

	test('hides the planned column in past weeks and carries overdue plans into the current week', () => {
		const overdue = flowTask({ id: 'overdue-plan', status: '예정', startDate: '2026-05-18', endDate: '2026-05-22' });
		const pastWeek = { weekStartISO: '2026-05-25', weekEndISO: '2026-05-31', weekPosition: 'past' as const };
		const currentWeek = { weekStartISO: '2026-06-01', weekEndISO: '2026-06-07', weekPosition: 'current' as const };

		expect(buildFlowTaskBoard([overdue], pastWeek).map((column) => column.status)).not.toContain('예정');
		expect(buildFlowTaskBoard([overdue], currentWeek).find((column) => column.status === '예정')?.tasks.map((task) => task.id)).toEqual([
			'overdue-plan'
		]);
	});

	test('shows requested tasks in every week and in-progress tasks only in the current week', () => {
		const tasks = [
			flowTask({ id: 'requested', status: '요청', startDate: '2026-05-04', endDate: '' }),
			flowTask({ id: 'in-progress', status: '진행', startDate: '2026-05-04', endDate: '' })
		];
		const pastWeek = { weekStartISO: '2026-06-01', weekEndISO: '2026-06-07', weekPosition: 'past' as const };
		const currentWeek = { ...pastWeek, weekPosition: 'current' as const };

		expect(buildFlowTaskBoard(tasks, pastWeek).flatMap((column) => column.tasks.map((task) => task.id))).toEqual(['requested']);
		expect(buildFlowTaskBoard(tasks, currentWeek).flatMap((column) => column.tasks.map((task) => task.id))).toEqual([
			'requested',
			'in-progress'
		]);
	});

	test('limits only the completed column to tasks completed in the selected week', () => {
		const board = buildFlowTaskBoard([
			flowTask({ id: 'done-this-week', status: '완료', endDate: '2026-06-03' }),
			flowTask({ id: 'done-last-week', status: '완료', endDate: '2026-05-28' }),
			flowTask({ id: 'progress-last-week', status: '진행', endDate: '2026-05-28' })
		], {
			weekStartISO: '2026-06-01',
			weekEndISO: '2026-06-07',
			weekPosition: 'current' as const
		});

		expect(board.find((column) => column.status === '완료')?.tasks.map((task) => task.id)).toEqual(['done-this-week']);
		expect(board.find((column) => column.status === '진행')?.tasks.map((task) => task.id)).toEqual(['progress-last-week']);
	});
});

function flowTask(overrides: Partial<FlowTask>): FlowTask {
	return {
		id: '',
		ownerID: 'member-1',
		ownerName: '김철수',
		participantIDs: ['member-1'],
		participantNames: ['김철수'],
		business: '여명거리',
		type: '기능',
		content: '업무',
		goal: '완료',
		size: 'M',
		status: '예정',
		statusRank: 0,
		weekCode: '26W23',
		flag: 0,
		...overrides
	};
}
