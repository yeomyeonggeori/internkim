import { describe, expect, mock, test } from 'bun:test';

const movedTaskRow = {
	title: '보고서 초안',
	note: null,
	location: null,
	business: null,
	type: null,
	size: 'M',
	is_event: false,
	is_whole_day: false,
	notify_minutes_before: null,
	updated_at: '2026-06-01T00:00:00Z',
	task_participant: [{ member_id: 'member-1' }]
};

const plane = {
	from: () => ({
		select: () => ({
			eq: () => ({
				single: async () => ({ data: movedTaskRow, error: null })
			})
		})
	}),
	rpc: async () => ({ data: null, error: null }),
	auth: { getSession: async () => ({ data: { session: null } }) },
	functions: { invoke: async () => ({ data: null, error: null }) }
};

mock.module('$lib/supabase', () => ({
	supabase: () => plane,
	isSupabaseConfigured: () => true,
	gatewayURL: () => '',
	projectURL: () => '',
	vapidPublicKey: () => ''
}));

const { saveTaskBoardMove } = await import('../../../src/routes/task/task-board-save');

describe('task board save', () => {
	test('keeps a saved board move when reload fails afterward', async () => {
		const messages: string[] = [];
		const loadOptions: unknown[] = [];

		const result = await saveTaskBoardMove({
			request: {
				taskID: 'requested',
				targetStatus: 'in_progress',
				beforeTaskID: null
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
		expect(loadOptions).toEqual([{ preserveActiveTabOnError: true }]);
		expect(messages).toEqual(['업무 데이터를 불러오지 못했습니다.']);
	});
});
