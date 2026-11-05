import { describe, expect, test } from 'bun:test';
import { BOARD_STATUS_VALUES, buildTaskBoard, isOverdueTaskPlan } from '../../src/routes/task/task-board-model';
import type { Task } from '../../src/routes/task/task-types';

describe('flow task board model', () => {
	test('groups only board statuses and excludes rejected or stopped tasks', () => {
		const board = buildTaskBoard([
			task({ id: 'requested-1', status: 'requested' }),
			task({ id: 'rejected-1', status: 'rejected' }),
			task({ id: 'stopped-1', status: 'stopped' }),
			task({ id: 'paused-1', status: 'paused' })
		]);

		expect(board.map((column) => column.status)).toEqual([...BOARD_STATUS_VALUES]);
		expect(board.flatMap((column) => column.tasks.map((task) => task.id))).toEqual(['requested-1', 'paused-1']);
	});

	test('sorts tasks from top to bottom by end date, earliest first', () => {
		const board = buildTaskBoard([
			task({ id: 'last', status: 'in_progress', endDate: '2026-06-10' }),
			task({ id: 'first', status: 'in_progress', endDate: '2026-06-01' }),
			task({ id: 'middle', status: 'in_progress', endDate: '2026-06-05' })
		]);

		const progressColumn = board.find((column) => column.status === 'in_progress');

		expect(progressColumn?.tasks.map((task) => task.id)).toEqual(['first', 'middle', 'last']);
	});

	test('puts parent tasks above other tasks, each group by end date', () => {
		const board = buildTaskBoard([
			task({ id: 'early-task', status: 'in_progress', endDate: '2026-06-01' }),
			task({ id: 'late-parent', status: 'in_progress', endDate: '2026-06-10' }),
			task({ id: 'early-parent', status: 'in_progress', endDate: '2026-06-05' })
		], { parentTaskIDs: new Set(['late-parent', 'early-parent']) });

		const progressColumn = board.find((column) => column.status === 'in_progress');

		expect(progressColumn?.tasks.map((task) => task.id)).toEqual(['early-parent', 'late-parent', 'early-task']);
	});

	test('breaks a tie on end date by creation time, earliest first', () => {
		const board = buildTaskBoard([
			task({ id: 'created-later', status: 'in_progress', endDate: '2026-06-05', createdAt: '2026-06-01T12:00:00Z' }),
			task({ id: 'created-first', status: 'in_progress', endDate: '2026-06-05', createdAt: '2026-06-01T09:00:00Z' })
		]);

		const progressColumn = board.find((column) => column.status === 'in_progress');

		expect(progressColumn?.tasks.map((task) => task.id)).toEqual(['created-first', 'created-later']);
	});

	test('sorts a task with no end date after every dated task in the column', () => {
		const board = buildTaskBoard([
			task({ id: 'undated', status: 'in_progress', endDate: '' }),
			task({ id: 'dated', status: 'in_progress', endDate: '2026-06-05' })
		]);

		const progressColumn = board.find((column) => column.status === 'in_progress');

		expect(progressColumn?.tasks.map((task) => task.id)).toEqual(['dated', 'undated']);
	});

	test('uses task id as a stable tie-break when end date and creation time match', () => {
		const board = buildTaskBoard([
			task({ id: 'task-c', status: 'in_progress' }),
			task({ id: 'task-a', status: 'in_progress' }),
			task({ id: 'task-b', status: 'in_progress' })
		]);

		const progressColumn = board.find((column) => column.status === 'in_progress');

		expect(progressColumn?.tasks.map((task) => task.id)).toEqual(['task-a', 'task-b', 'task-c']);
	});

	test('shows a planned task in every week from its start week to its end week', () => {
		const tasks = [
			task({ id: 'ends-this-week', status: 'planned', startDate: '2026-05-25', endDate: '2026-06-03' }),
			task({ id: 'spans-into-next-week', status: 'planned', startDate: '2026-06-02', endDate: '2026-06-10' }),
			task({ id: 'ends-in-two-weeks', status: 'planned', startDate: '', endDate: '2026-06-17' }),
			task({ id: 'starts-next-week', status: 'planned', startDate: '2026-06-08', endDate: '2026-06-12' }),
			task({ id: 'starts-next-week-open-ended', status: 'planned', startDate: '2026-06-09', endDate: '' }),
			task({ id: 'started-open-ended', status: 'planned', startDate: '2026-05-20', endDate: '' }),
			task({ id: 'undated', status: 'planned', startDate: '', endDate: '' })
		];
		const plannedIn = (weekStartISO: string, weekEndISO: string, weekPosition: 'current' | 'future') =>
			buildTaskBoard(tasks, { weekStartISO, weekEndISO, weekPosition })
				.find((column) => column.status === 'planned')
				?.tasks.map((task) => task.id)
				.sort();

		expect(plannedIn('2026-06-01', '2026-06-07', 'current')).toEqual(
			['ends-in-two-weeks', 'ends-this-week', 'spans-into-next-week', 'started-open-ended', 'undated'].sort()
		);
		expect(plannedIn('2026-06-08', '2026-06-14', 'future')).toEqual(
			['ends-in-two-weeks', 'spans-into-next-week', 'started-open-ended', 'starts-next-week', 'starts-next-week-open-ended', 'undated'].sort()
		);
		expect(plannedIn('2026-06-15', '2026-06-21', 'future')).toEqual(
			['ends-in-two-weeks', 'started-open-ended', 'starts-next-week-open-ended', 'undated'].sort()
		);
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
		ownerName: '이샘플',
		participantIDs: ['member-1'],
		participantNames: ['이샘플'],
		business: '샘플거리',
		type: '기능',
		content: '업무',
		size: 'M',
		status: 'planned',
		weekCode: '26W23',
		...overrides
	};
}
