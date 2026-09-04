import { describe, expect, test } from 'bun:test';
import { createTaskBoardMove } from '../../src/routes/task/task-board-drag';
import type { Task } from '../../src/routes/task/task-types';

describe('flow task board drag', () => {
	test('moves a task to another status column', () => {
		const result = createTaskBoardMove([
			task({ id: 'requested', status: 'requested' }),
			task({ id: 'progress-a', status: 'in_progress' })
		], {
			taskID: 'requested',
			targetStatus: 'in_progress'
		});

		expect(result?.updates.map((task) => [task.id, task.status])).toEqual([
			['requested', 'in_progress']
		]);
		expect(result?.tasks.find((task) => task.id === 'requested')?.status).toBe('in_progress');
	});

	test('ignores a drop that keeps the task in the same status column', () => {
		const result = createTaskBoardMove([
			task({ id: 'task-a', status: 'in_progress' }),
			task({ id: 'task-b', status: 'in_progress' })
		], {
			taskID: 'task-a',
			targetStatus: 'in_progress'
		});

		expect(result).toBe(null);
	});

	test('ignores unknown tasks or non-board statuses', () => {
		expect(createTaskBoardMove([], {
			taskID: 'missing',
			targetStatus: 'in_progress'
		})).toBe(null);
		expect(createTaskBoardMove([
			task({ id: 'task-a', status: 'in_progress' })
		], {
			taskID: 'task-a',
			targetStatus: 'rejected'
		})).toBe(null);
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
		weekCode: '26W23',
		...overrides
	};
}
