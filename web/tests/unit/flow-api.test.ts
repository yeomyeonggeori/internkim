// Flow API 오류 문구 로컬라이제이션 경계를 검증합니다.
import { describe, expect, test } from 'bun:test';
import { createQuickFlowTask, fetchFlowSummary } from '../../src/routes/flow/flow-api';

type FetchWithPreconnect = typeof fetch & { preconnect?: unknown };

function fetchPreconnect(fetchValue: typeof fetch) {
	return (fetchValue as FetchWithPreconnect).preconnect;
}

describe('flow API', () => {
	test('returns skipped duplicate quick task responses', async () => {
		const originalFetch = globalThis.fetch;

		try {
			globalThis.fetch = Object.assign(
				async (): Promise<Response> =>
					Response.json({
						status: 'skipped_duplicate',
						reason: 'same date and content'
					}),
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			const result = await createQuickFlowTask(
				{
					prompt: '10-minute meeting',
					ownerID: 'member-1',
					participantIDs: ['member-1'],
					weekCode: '26W23'
				},
				'Could not add the task with AI.'
			);

			expect(result.status).toBe('skipped_duplicate');
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('sends allowDuplicate when confirming a duplicate quick task', async () => {
		const originalFetch = globalThis.fetch;
		let requestBody: unknown = null;

		try {
			globalThis.fetch = Object.assign(
				async (_input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
					requestBody = JSON.parse(String(init?.body ?? '{}'));
					return Response.json({ id: 'task-1' });
				},
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			const result = await createQuickFlowTask(
				{
					prompt: '10-minute meeting',
					ownerID: 'member-1',
					participantIDs: ['member-1'],
					weekCode: '26W23',
					allowDuplicate: true
				},
				'Could not add the task with AI.'
			);

			expect(result.status).toBe('created');
			expect(requestBody).toEqual({
				prompt: '10-minute meeting',
				ownerID: 'member-1',
				participantIDs: ['member-1'],
				weekCode: '26W23',
				allowDuplicate: true
			});
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('uses localized fallback instead of raw server error copy', async () => {
		const originalFetch = globalThis.fetch;

		try {
			globalThis.fetch = Object.assign(
				async (): Promise<Response> => new Response('이미 추가된 업무입니다.', { status: 409 }),
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			await expect(
				createQuickFlowTask(
					{
						prompt: '10-minute meeting',
						ownerID: 'member-1',
						participantIDs: ['member-1'],
						weekCode: '26W23'
					},
					'Could not add the task with AI.'
				)
			).rejects.toThrow('Could not add the task with AI.');
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('keeps empty week query out of the summary request', async () => {
		const originalFetch = globalThis.fetch;
		let requestedURL = '';

		try {
			globalThis.fetch = Object.assign(
				async (input: RequestInfo | URL): Promise<Response> => {
					requestedURL = String(input);
					return Response.json({
						week: {
							code: '26W23',
							startISO: '2026-06-01',
							endISO: '2026-06-07',
							previous: '26W22',
							next: '26W24',
							isCurrent: true
						},
						currentUserEmail: 'member@example.com',
						isAdmin: false,
						members: [],
						tasks: [],
						definitions: { categories: [], types: [], sizes: [] },
						statusOptions: [],
						metrics: {}
					});
				},
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			await fetchFlowSummary('', 'Could not load Flow data.');

			expect(requestedURL).toBe('/flow/api/summary');
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});
