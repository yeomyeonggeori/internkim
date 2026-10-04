import { afterAll, describe, expect, mock, test } from 'bun:test';

const centralPlane = { ...(await import('$lib/supabase')) };
const publicAPICall = { ...(await import('../../../src/lib/public-api-call')) };
const written: { name: string; input: Record<string, unknown> }[] = [];

const plane = {
	auth: { getSession: async () => ({ data: { session: null } }) },
	functions: { invoke: async () => ({ data: null, error: null }) }
};

mock.module('$lib/supabase', () => ({
	...centralPlane,
	supabase: () => plane,
	isSupabaseConfigured: () => true
}));

mock.module('../../../src/lib/public-api-call', () => ({
	...publicAPICall,
	invokeTool: async (name: string, input: Record<string, unknown>) => {
		written.push({ name, input });
		return { taskID: 'requested' };
	}
}));

const { saveTaskBoardMove } = await import('../../../src/routes/task/task-board-save');

afterAll(() => {
	mock.module('$lib/supabase', () => centralPlane);
	mock.module('../../../src/lib/public-api-call', () => publicAPICall);
});

describe('task board save', () => {
	test('keeps a saved board move when reload fails afterward', async () => {
		const messages: string[] = [];
		const loadOptions: unknown[] = [];

		const result = await saveTaskBoardMove({
			request: {
				taskID: 'requested',
				targetStatus: 'in_progress'
			},
			week: '26W23',
			currentWeek: () => '26W23',
			loadTask: async (_week: string, options?: unknown) => {
				loadOptions.push(options);
				return false;
			},
			setPageErrorMessage: (message) => messages.push(message),
			saveErrorMessage: '업무를 저장하지 못했습니다.',
			loadErrorMessage: '업무 데이터를 불러오지 못했습니다.'
		});

		expect(result).toBe('saved_with_reload_error');
		expect(written).toEqual([{ name: 'task_update', input: { taskHint: 'requested', status: 'in_progress' } }]);
		expect(loadOptions).toEqual([{ preserveActiveTabOnError: true }]);
		expect(messages).toEqual(['업무 데이터를 불러오지 못했습니다.']);
	});
});
