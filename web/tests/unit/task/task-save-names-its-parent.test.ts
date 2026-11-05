import { afterAll, beforeEach, describe, expect, mock, test } from 'bun:test';
import type { Task } from '../../../src/routes/task/task-types';

let invoked: { tool: string; input: Record<string, unknown> }[] = [];

const publicAPICall = { ...(await import('../../../src/lib/public-api-call')) };

mock.module('../../../src/lib/public-api-call', () => ({
	...publicAPICall,
	invokeTool: async (name: string, input: Record<string, unknown>) => {
		invoked.push({ tool: name, input });
		return { taskID: 'task-1' };
	}
}));

const { saveTask } = await import('../../../src/lib/task/task-state');

afterAll(() => {
	mock.module('../../../src/lib/public-api-call', () => publicAPICall);
});

function taskWith(fields: Partial<Task> = {}): Task {
	return {
		id: 'task-1',
		ownerID: 'member-1',
		ownerName: '이샘플',
		participantIDs: ['member-2'],
		participantNames: ['박예시'],
		business: null,
		type: null,
		content: '보고서 초안',
		size: 'M',
		status: 'in_progress',
		weekCode: '2026-W36',
		...fields
	};
}

beforeEach(() => {
	invoked = [];
});

describe('a task saved from the screen', () => {
	test('a new task under a parent names the parent it goes under', async () => {
		await saveTask(taskWith({ id: '', parentTaskID: 'parent-1' }));

		expect(invoked.map((call) => call.tool)).toEqual(['task_add']);
		expect(invoked[0].input.parentTaskHint).toBe('parent-1');
	});

	test('an existing task names itself and never its parent, which relationships own', async () => {
		await saveTask(taskWith({ parentTaskID: 'parent-1' }));

		expect(invoked.map((call) => call.tool)).toEqual(['task_update']);
		expect(invoked[0].input.taskHint).toBe('task-1');
		expect('parentTaskHint' in invoked[0].input).toBe(false);
	});
});
