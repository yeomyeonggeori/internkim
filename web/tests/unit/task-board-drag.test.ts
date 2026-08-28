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
			task({ id: 'requested', status: '요청', statusRank: taskBoardRankStep }),
			task({ id: 'progress-a', status: '진행', statusRank: taskBoardRankStep }),
			task({ id: 'progress-b', status: '진행', statusRank: taskBoardRankStep * 2 })
		], {
			taskID: 'requested',
			targetStatus: '진행',
			beforeTaskID: null
		});

		expect(result?.updates.map((task) => [task.id, task.status, task.statusRank])).toEqual([
			['requested', '진행', taskBoardRankStep * 3]
		]);
		expect(boardTaskIDs(result?.tasks ?? [], '진행')).toEqual(['progress-a', 'progress-b', 'requested']);
	});

	test('moves a task between neighbors with one persisted update when rank space exists', () => {
		const result = createTaskBoardMove([
			task({ id: 'requested', status: '요청', statusRank: taskBoardRankStep }),
			task({ id: 'progress-a', status: '진행', statusRank: taskBoardRankStep }),
			task({ id: 'progress-b', status: '진행', statusRank: taskBoardRankStep * 3 })
		], {
			taskID: 'requested',
			targetStatus: '진행',
			beforeTaskID: 'progress-b'
		});

		expect(result?.updates.map((task) => [task.id, task.status, task.statusRank])).toEqual([
			['requested', '진행', taskBoardRankStep * 2]
		]);
		expect(boardTaskIDs(result?.tasks ?? [], '진행')).toEqual(['progress-a', 'requested', 'progress-b']);
	});

	test('reorders inside the same status with one persisted update when rank space exists', () => {
		const result = createTaskBoardMove([
			task({ id: 'task-a', status: '진행', statusRank: taskBoardRankStep }),
			task({ id: 'task-b', status: '진행', statusRank: taskBoardRankStep * 2 }),
			task({ id: 'task-c', status: '진행', statusRank: taskBoardRankStep * 3 })
		], {
			taskID: 'task-c',
			targetStatus: '진행',
			beforeTaskID: 'task-a'
		});

		expect(result?.updates.map((task) => [task.id, task.statusRank])).toEqual([
			['task-c', taskBoardRankStep / 2]
		]);
		expect(boardTaskIDs(result?.tasks ?? [], '진행')).toEqual(['task-c', 'task-a', 'task-b']);
	});

	test('returns the moved task first when rank space is exhausted and the target column needs normalization', () => {
		const result = createTaskBoardMove([
			task({ id: 'requested', status: '요청', statusRank: taskBoardRankStep }),
			task({ id: 'progress-a', status: '진행', statusRank: 1 }),
			task({ id: 'progress-b', status: '진행', statusRank: 2 })
		], {
			taskID: 'requested',
			targetStatus: '진행',
			beforeTaskID: 'progress-b'
		});

		expect(result?.updates.map((task) => task.id)).toEqual(['requested', 'progress-a', 'progress-b']);
	});

	test('reorders tasks inside the same status column with stable ranks', () => {
		const result = createTaskBoardMove([
			task({ id: 'task-a', status: '진행', statusRank: 0 }),
			task({ id: 'task-b', status: '진행', statusRank: 0 }),
			task({ id: 'task-c', status: '진행', statusRank: 0 })
		], {
			taskID: 'task-c',
			targetStatus: '진행',
			beforeTaskID: 'task-a'
		});

		expect(result?.updates.map((task) => [task.id, task.statusRank])).toEqual([
			['task-c', taskBoardRankStep],
			['task-a', taskBoardRankStep * 2],
			['task-b', taskBoardRankStep * 3]
		]);
		expect(boardTaskIDs(result?.tasks ?? [], '진행')).toEqual(['task-c', 'task-a', 'task-b']);
	});

	test('ignores drops that keep the same task order', () => {
		const result = createTaskBoardMove([
			task({ id: 'task-a', status: '진행', statusRank: taskBoardRankStep }),
			task({ id: 'task-b', status: '진행', statusRank: taskBoardRankStep * 2 }),
			task({ id: 'task-c', status: '진행', statusRank: taskBoardRankStep * 3 })
		], {
			taskID: 'task-b',
			targetStatus: '진행',
			beforeTaskID: 'task-c'
		});

		expect(result).toBe(null);
	});

	test('ignores adjacent drops that target the moved task as the insertion boundary', () => {
		const result = createTaskBoardMove([
			task({ id: 'task-a', status: '진행', statusRank: taskBoardRankStep }),
			task({ id: 'task-b', status: '진행', statusRank: taskBoardRankStep * 2 }),
			task({ id: 'task-c', status: '진행', statusRank: taskBoardRankStep * 3 })
		], {
			taskID: 'task-b',
			targetStatus: '진행',
			beforeTaskID: 'task-b'
		});

		expect(result).toBe(null);
	});

	test('ignores unknown tasks or non-board statuses', () => {
		expect(createTaskBoardMove([], {
			taskID: 'missing',
			targetStatus: '진행',
			beforeTaskID: null
		})).toBe(null);
		expect(createTaskBoardMove([
			task({ id: 'task-a', status: '진행', statusRank: taskBoardRankStep })
		], {
			taskID: 'task-a',
			targetStatus: '기각',
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
		status: '예정',
		statusRank: 0,
		weekCode: '26W23',
		...overrides
	};
}
