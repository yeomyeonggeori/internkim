import { describe, expect, test } from 'bun:test';
import {
	createTaskBoardMove,
	taskBoardRankStep
} from '../../src/routes/task/task-board-drag';
import { buildTaskBoard } from '../../src/routes/task/task-board-model';
import type { Task } from '../../src/routes/task/task-types';

describe('flow task board drag', () => {
	test('moves a task to the bottom of another status column', () => {
		const result = createTaskBoardMove([
			task({ id: 'requested', status: 'requested', statusRank: taskBoardRankStep }),
			task({ id: 'progress-a', status: 'in_progress', statusRank: taskBoardRankStep }),
			task({ id: 'progress-b', status: 'in_progress', statusRank: taskBoardRankStep * 2 })
		], {
			taskID: 'requested',
			targetStatus: 'in_progress',
			beforeTaskID: null
		});

		expect(result?.updates.map((task) => [task.id, task.status, task.statusRank])).toEqual([
			['requested', 'in_progress', taskBoardRankStep * 3]
		]);
		expect(boardTaskIDs(result?.tasks ?? [], 'in_progress')).toEqual(['progress-a', 'progress-b', 'requested']);
	});

	test('moves a task between neighbors with one persisted update when rank space exists', () => {
		const result = createTaskBoardMove([
			task({ id: 'requested', status: 'requested', statusRank: taskBoardRankStep }),
			task({ id: 'progress-a', status: 'in_progress', statusRank: taskBoardRankStep }),
			task({ id: 'progress-b', status: 'in_progress', statusRank: taskBoardRankStep * 3 })
		], {
			taskID: 'requested',
			targetStatus: 'in_progress',
			beforeTaskID: 'progress-b'
		});

		expect(result?.updates.map((task) => [task.id, task.status, task.statusRank])).toEqual([
			['requested', 'in_progress', taskBoardRankStep * 2]
		]);
		expect(boardTaskIDs(result?.tasks ?? [], 'in_progress')).toEqual(['progress-a', 'requested', 'progress-b']);
	});

	test('reorders inside the same status with one persisted update when rank space exists', () => {
		const result = createTaskBoardMove([
			task({ id: 'task-a', status: 'in_progress', statusRank: taskBoardRankStep }),
			task({ id: 'task-b', status: 'in_progress', statusRank: taskBoardRankStep * 2 }),
			task({ id: 'task-c', status: 'in_progress', statusRank: taskBoardRankStep * 3 })
		], {
			taskID: 'task-c',
			targetStatus: 'in_progress',
			beforeTaskID: 'task-a'
		});

		expect(result?.updates.map((task) => [task.id, task.statusRank])).toEqual([
			['task-c', taskBoardRankStep / 2]
		]);
		expect(boardTaskIDs(result?.tasks ?? [], 'in_progress')).toEqual(['task-c', 'task-a', 'task-b']);
	});

	test('returns the moved task first when rank space is exhausted and the target column needs normalization', () => {
		const result = createTaskBoardMove([
			task({ id: 'requested', status: 'requested', statusRank: taskBoardRankStep }),
			task({ id: 'progress-a', status: 'in_progress', statusRank: 1 }),
			task({ id: 'progress-b', status: 'in_progress', statusRank: 2 })
		], {
			taskID: 'requested',
			targetStatus: 'in_progress',
			beforeTaskID: 'progress-b'
		});

		expect(result?.updates.map((task) => task.id)).toEqual(['requested', 'progress-a', 'progress-b']);
	});

	test('reorders tasks inside the same status column with stable ranks', () => {
		const result = createTaskBoardMove([
			task({ id: 'task-a', status: 'in_progress', statusRank: 0 }),
			task({ id: 'task-b', status: 'in_progress', statusRank: 0 }),
			task({ id: 'task-c', status: 'in_progress', statusRank: 0 })
		], {
			taskID: 'task-c',
			targetStatus: 'in_progress',
			beforeTaskID: 'task-a'
		});

		expect(result?.updates.map((task) => [task.id, task.statusRank])).toEqual([
			['task-c', taskBoardRankStep],
			['task-a', taskBoardRankStep * 2],
			['task-b', taskBoardRankStep * 3]
		]);
		expect(boardTaskIDs(result?.tasks ?? [], 'in_progress')).toEqual(['task-c', 'task-a', 'task-b']);
	});

	test('ignores drops that keep the same task order', () => {
		const result = createTaskBoardMove([
			task({ id: 'task-a', status: 'in_progress', statusRank: taskBoardRankStep }),
			task({ id: 'task-b', status: 'in_progress', statusRank: taskBoardRankStep * 2 }),
			task({ id: 'task-c', status: 'in_progress', statusRank: taskBoardRankStep * 3 })
		], {
			taskID: 'task-b',
			targetStatus: 'in_progress',
			beforeTaskID: 'task-c'
		});

		expect(result).toBe(null);
	});

	test('ignores adjacent drops that target the moved task as the insertion boundary', () => {
		const result = createTaskBoardMove([
			task({ id: 'task-a', status: 'in_progress', statusRank: taskBoardRankStep }),
			task({ id: 'task-b', status: 'in_progress', statusRank: taskBoardRankStep * 2 }),
			task({ id: 'task-c', status: 'in_progress', statusRank: taskBoardRankStep * 3 })
		], {
			taskID: 'task-b',
			targetStatus: 'in_progress',
			beforeTaskID: 'task-b'
		});

		expect(result).toBe(null);
	});

	test('ignores unknown tasks or non-board statuses', () => {
		expect(createTaskBoardMove([], {
			taskID: 'missing',
			targetStatus: 'in_progress',
			beforeTaskID: null
		})).toBe(null);
		expect(createTaskBoardMove([
			task({ id: 'task-a', status: 'in_progress', statusRank: taskBoardRankStep })
		], {
			taskID: 'task-a',
			targetStatus: 'rejected',
			beforeTaskID: null
		})).toBe(null);
	});
});

function boardTaskIDs(tasks: Task[], status: string): string[] {
	const column = buildTaskBoard(tasks).find((value) => value.status === status);
	return column?.tasks.map((task) => task.id) ?? [];
}

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
