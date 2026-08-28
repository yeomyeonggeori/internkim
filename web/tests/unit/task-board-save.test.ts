import { describe, expect, test } from 'bun:test';
import { saveTaskBoardMove } from '../../src/routes/task/task-board-save';

type FetchWithPreconnect = typeof fetch & { preconnect?: unknown };

function fetchPreconnect(fetchValue: typeof fetch) {
	return (fetchValue as FetchWithPreconnect).preconnect;
}

describe('flow task board save', () => {
	test('keeps a saved board move when reload fails afterward', async () => {
		const originalFetch = globalThis.fetch;
		const messages: string[] = [];
		const loadOptions: unknown[] = [];

		try {
			globalThis.fetch = Object.assign(
				async (): Promise<Response> => new Response(null, { status: 204 }),
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			const result = await saveTaskBoardMove({
				request: {
					taskID: 'requested',
					targetStatus: '진행',
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
			expect(loadOptions).toEqual([
				{
					preserveActiveTabOnError: true
				}
			]);
			expect(messages).toEqual([
				'업무 데이터를 불러오지 못했습니다.'
			]);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});
