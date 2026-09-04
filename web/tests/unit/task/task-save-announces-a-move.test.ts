import { beforeEach, describe, expect, mock, test } from 'bun:test';
import type { Task } from '../../../src/routes/task/task-types';

let announced: { name: string; body: unknown }[] = [];
let saved: Record<string, unknown>[] = [];

const plane = {
	rpc: async (name: string, args: Record<string, unknown>) => {
		saved.push({ name, ...args });
		return { data: null, error: null };
	},
	auth: { getSession: async () => ({ data: { session: { access_token: 'a-token' } } }) },
	functions: {
		invoke: async (name: string, options: { body: unknown }) => {
			announced.push({ name, body: options.body });
			return { data: null, error: null };
		}
	}
};

mock.module('$lib/supabase', () => ({ supabase: () => plane, isSupabaseConfigured: () => true }));

const { saveSupabaseTask } = await import('../../../src/lib/task/supabase-task');

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
	saved = [];
});

describe('a task saved with the status it was opened at', () => {
	test('announces the move when the saved status differs from the one it was opened at', async () => {
		await saveSupabaseTask(taskWith({ status: 'in_progress' }), 'planned');
		await afterTheSaveSettles();

		expect(saved).toHaveLength(1);
		expect(announced).toEqual([{ name: 'announce-task', body: { taskID: 'task-1' } }]);
	});

	test('says nothing when only the title changed', async () => {
		await saveSupabaseTask(taskWith({ status: 'in_progress', content: '보고서 최종본' }), 'in_progress');
		await afterTheSaveSettles();

		expect(saved).toHaveLength(1);
		expect(announced).toEqual([]);
	});

	test('says nothing about a task being created, which is nobody moving anything', async () => {
		await saveSupabaseTask(taskWith({ id: '' }), null);
		await afterTheSaveSettles();

		expect(saved).toHaveLength(1);
		expect(announced).toEqual([]);
	});
});
