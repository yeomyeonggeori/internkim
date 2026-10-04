import { afterAll, beforeEach, describe, expect, mock, test } from 'bun:test';
import type { Task } from '../../../src/routes/task/task-types';

let announced: { name: string; body: unknown }[] = [];
let invoked: { tool: string; input: Record<string, unknown> }[] = [];

const centralPlane = { ...(await import('$lib/supabase')) };
const publicAPICall = { ...(await import('../../../src/lib/public-api-call')) };

const plane = {
	auth: { getSession: async () => ({ data: { session: { access_token: 'a-token' } } }) },
	functions: {
		invoke: async (name: string, options: { body: unknown }) => {
			announced.push({ name, body: options.body });
			return { data: null, error: null };
		}
	}
};

mock.module('$lib/supabase', () => ({ ...centralPlane, supabase: () => plane, isSupabaseConfigured: () => true }));

mock.module('../../../src/lib/public-api-call', () => ({
	...publicAPICall,
	invokeTool: async (name: string, input: Record<string, unknown>) => {
		invoked.push({ tool: name, input });
		return { taskID: 'task-1' };
	}
}));

const { saveTask } = await import('../../../src/lib/task/task-state');

afterAll(() => {
	mock.module('$lib/supabase', () => centralPlane);
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

async function afterTheSaveSettles(): Promise<void> {
	await new Promise((resolve) => setTimeout(resolve, 0));
}

beforeEach(() => {
	announced = [];
	invoked = [];
});

describe('a task saved with the status it was opened at', () => {
	test('announces the move when the saved status differs from the one it was opened at', async () => {
		await saveTask(taskWith({ status: 'in_progress' }), 'planned');
		await afterTheSaveSettles();

		expect(invoked.map((call) => call.tool)).toEqual(['task_update']);
		expect(announced).toEqual([{ name: 'announce-task', body: { taskID: 'task-1' } }]);
	});

	test('says nothing when only the title changed', async () => {
		await saveTask(taskWith({ status: 'in_progress', content: '보고서 최종본' }), 'in_progress');
		await afterTheSaveSettles();

		expect(invoked).toHaveLength(1);
		expect(announced).toEqual([]);
	});

	test('says nothing about a task being created, which is nobody moving anything', async () => {
		await saveTask(taskWith({ id: '' }), null);
		await afterTheSaveSettles();

		expect(invoked.map((call) => call.tool)).toEqual(['task_add']);
		expect(announced).toEqual([]);
	});

	test('a new task under a parent names the parent it goes under', async () => {
		await saveTask(taskWith({ id: '', parentTaskID: 'parent-1' }), null);

		expect(invoked[0].input.parentTaskHint).toBe('parent-1');
	});

	test('an existing task names itself and never its parent, which relationships own', async () => {
		await saveTask(taskWith({ parentTaskID: 'parent-1' }), null);

		expect(invoked[0].input.taskHint).toBe('task-1');
		expect('parentTaskHint' in invoked[0].input).toBe(false);
	});
});
