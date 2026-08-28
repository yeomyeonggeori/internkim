import { describe, expect, test } from 'bun:test';
import { BOARD_STATUS_VALUES, buildTaskBoard, isOverdueTaskPlan } from '../../src/routes/task/task-board-model';
import type { Task } from '../../src/routes/task/task-types';

describe('flow task board model', () => {
	test('groups only board statuses and excludes rejected or stopped tasks', () => {
		const board = buildTaskBoard([
			task({ id: 'requested-1', status: 'requested', statusRank: 100 }),
			task({ id: 'rejected-1', status: 'rejected', statusRank: 100 }),
			task({ id: 'stopped-1', status: 'stopped', statusRank: 100 }),
			task({ id: 'paused-1', status: 'paused', statusRank: 100 })
		]);

		expect(board.map((column) => column.status)).toEqual(BOARD_STATUS_VALUES);
		expect(board.flatMap((column) => column.tasks.map((task) => task.id))).toEqual(['requested-1', 'paused-1']);
	});

	test('sorts tasks from top to bottom by status rank inside each column', () => {
		const board = buildTaskBoard([
			task({ id: 'bottom', status: 'in_progress', statusRank: 300 }),
			task({ id: 'top', status: 'in_progress', statusRank: 100 }),
			task({ id: 'middle', status: 'in_progress', statusRank: 200 })
		]);

		const progressColumn = board.find((column) => column.status === 'in_progress');

		expect(progressColumn?.tasks.map((task) => task.id)).toEqual(['top', 'middle', 'bottom']);
	});

	test('uses task id as a stable tie-break when status ranks match', () => {
		const board = buildTaskBoard([
			task({ id: 'task-c', status: 'in_progress', statusRank: 100 }),
			task({ id: 'task-a', status: 'in_progress', statusRank: 100 }),
			task({ id: 'task-b', status: 'in_progress', statusRank: 100 })
		]);

		const progressColumn = board.find((column) => column.status === 'in_progress');

		expect(progressColumn?.tasks.map((task) => task.id)).toEqual(['task-a', 'task-b', 'task-c']);
	});

	test('assigns a visible accent theme to every board column status', () => {
		const board = buildTaskBoard([]);

		expect(board.map((column) => [column.status, column.theme.dotClass])).toEqual([
			['requested', 'bg-[#7c3aed]'],
			['planned', 'bg-[#d97706]'],
			['in_progress', 'bg-[#0284c7]'],
			['completed', 'bg-[#16a34a]'],
			['paused', 'bg-[#e11d48]']
		]);
	});

	test('keeps planned tasks in the week of their end date, or their start date when open ended', () => {
		const board = buildTaskBoard([
			task({ id: 'ends-this-week', status: 'planned', startDate: '2026-05-25', endDate: '2026-06-03' }),
			task({ id: 'ends-next-week', status: 'planned', startDate: '2026-06-02', endDate: '2026-06-10' }),
			task({ id: 'starts-this-week-open-ended', status: 'planned', startDate: '2026-06-04', endDate: '' })
		], {
			weekStartISO: '2026-06-01',
			weekEndISO: '2026-06-07',
			weekPosition: 'future'
		});

		expect(board.find((column) => column.status === 'planned')?.tasks.map((task) => task.id)).toEqual([
			'ends-this-week',
			'starts-this-week-open-ended'
		]);
	});

	test('hides the planned column in past weeks and carries overdue plans into the current week', () => {
		const overdue = task({ id: 'overdue-plan', status: 'planned', startDate: '2026-05-18', endDate: '2026-05-22' });
		const pastWeek = { weekStartISO: '2026-05-25', weekEndISO: '2026-05-31', weekPosition: 'past' as const };
		const currentWeek = { weekStartISO: '2026-06-01', weekEndISO: '2026-06-07', weekPosition: 'current' as const };

		expect(buildTaskBoard([overdue], pastWeek).map((column) => column.status)).not.toContain('planned');
		expect(buildTaskBoard([overdue], currentWeek).find((column) => column.status === 'planned')?.tasks.map((task) => task.id)).toEqual([
			'overdue-plan'
		]);
	});

	test('shows requested tasks in every week and in-progress tasks only in the current week', () => {
		const tasks = [
			task({ id: 'requested', status: 'requested', startDate: '2026-05-04', endDate: '' }),
			task({ id: 'in-progress', status: 'in_progress', startDate: '2026-05-04', endDate: '' })
		];
		const pastWeek = { weekStartISO: '2026-06-01', weekEndISO: '2026-06-07', weekPosition: 'past' as const };
		const currentWeek = { ...pastWeek, weekPosition: 'current' as const };

		expect(buildTaskBoard(tasks, pastWeek).flatMap((column) => column.tasks.map((task) => task.id))).toEqual(['requested']);
		expect(buildTaskBoard(tasks, currentWeek).flatMap((column) => column.tasks.map((task) => task.id))).toEqual([
			'requested',
			'in-progress'
		]);
	});

	test('marks a plan as overdue only when its date falls before the selected week', () => {
		expect(isOverdueTaskPlan(task({ status: 'planned', startDate: '2026-05-18', endDate: '2026-05-22' }), '2026-06-01')).toBe(true);
		expect(isOverdueTaskPlan(task({ status: 'planned', startDate: '2026-05-18', endDate: '' }), '2026-06-01')).toBe(true);
		expect(isOverdueTaskPlan(task({ status: 'planned', startDate: '2026-06-02', endDate: '2026-06-05' }), '2026-06-01')).toBe(false);
		expect(isOverdueTaskPlan(task({ status: 'in_progress', startDate: '2026-05-18', endDate: '2026-05-22' }), '2026-06-01')).toBe(false);
	});

	test('hides an empty request column when asked, and keeps it when it has tasks', () => {
		const empty = buildTaskBoard([], { hideEmptyRequestColumn: true });
		const filled = buildTaskBoard([task({ id: 'requested', status: 'requested' })], { hideEmptyRequestColumn: true });

		expect(empty.map((column) => column.status)).not.toContain('requested');
		expect(filled.map((column) => column.status)).toContain('requested');
	});

	test('limits only the completed column to tasks completed in the selected week', () => {
		const board = buildTaskBoard([
			task({ id: 'done-this-week', status: 'completed', endDate: '2026-06-03' }),
			task({ id: 'done-last-week', status: 'completed', endDate: '2026-05-28' }),
			task({ id: 'progress-last-week', status: 'in_progress', endDate: '2026-05-28' })
		], {
			weekStartISO: '2026-06-01',
			weekEndISO: '2026-06-07',
			weekPosition: 'current' as const
		});

		expect(board.find((column) => column.status === 'completed')?.tasks.map((task) => task.id)).toEqual(['done-this-week']);
		expect(board.find((column) => column.status === 'in_progress')?.tasks.map((task) => task.id)).toEqual(['progress-last-week']);
	});
});

function task(overrides: Partial<Task>): Task {
	return {
		id: '',
		ownerID: 'member-1',
		ownerName: '김철수',
		participantIDs: ['member-1'],
		participantNames: ['김철수'],
		business: '샘플거리',
		type: '기능',
		content: '업무',
		size: 'M',
		status: 'planned',
		statusRank: 0,
		weekCode: '26W23',
		...overrides
	};
}
