// Flow API 오류 문구 로컬라이제이션 경계를 검증합니다.
import { describe, expect, test } from 'bun:test';
import { createQuickFlowTask, fetchFlowState, fetchFlowWeeklySummary, mergeFlowSummary } from '../../src/routes/flow/flow-api';

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
				{ preconnect: originalFetch.preconnect }
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
				{ preconnect: originalFetch.preconnect }
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
				{ preconnect: originalFetch.preconnect }
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
				{ preconnect: originalFetch.preconnect }
			);

			await fetchFlowWeeklySummary('', 'Could not load Flow data.');

			expect(requestedURL).toBe('/flow/api/summary');
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('loads global state from the state request', async () => {
		const originalFetch = globalThis.fetch;
		let requestedURL = '';

		try {
			globalThis.fetch = Object.assign(
				async (input: RequestInfo | URL): Promise<Response> => {
					requestedURL = String(input);
					return Response.json({
						currentWeek: {
							code: '26W23',
							startISO: '2026-06-01',
							endISO: '2026-06-07',
							previous: '26W22',
							next: '26W24',
							isCurrent: true
						},
						currentUserEmail: 'member@example.com',
						currentUserName: 'Member',
						isAdmin: false,
						members: [],
						tasks: [],
						definitions: { categories: [], types: [], sizes: [] },
						statusOptions: [],
						metrics: {
							totalTasks: 0,
							completedTasks: 0,
							requestedTasks: 0,
							pausedTasks: 0,
							stoppedTasks: 0,
							statusCounts: {},
							businessCounts: {},
							typeCounts: {},
							memberScores: {},
							memberScoreDetails: {}
						}
					});
				},
				{ preconnect: originalFetch.preconnect }
			);

			await fetchFlowState('Could not load Flow data.');

			expect(requestedURL).toBe('/flow/api/state');
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('merges weekly metrics with current member scores from state', () => {
		const week = {
			code: '26W23',
			startISO: '2026-06-01',
			endISO: '2026-06-07',
			previous: '26W22',
			next: '26W24',
			isCurrent: true
		};
		const state = {
			currentWeek: week,
			currentUserEmail: 'member@example.com',
			currentUserName: 'Member',
			isAdmin: false,
			members: [],
			tasks: [flowTask('global-task')],
			definitions: { categories: [], types: [], sizes: [] },
			statusOptions: [],
			source: 'sqlite',
			metrics: {
				totalTasks: 0,
				completedTasks: 0,
				requestedTasks: 0,
				pausedTasks: 0,
				stoppedTasks: 0,
				statusCounts: {},
				businessCounts: {},
				typeCounts: {},
				memberScores: { 'member-1': 42 },
				memberScoreDetails: { 'member-1': { weeklyScore: 10, monthlyScore: 32, currentScore: 42 } },
				totalScore: 42
			}
		};
		const weeklySummary = {
			week,
			currentWeek: week,
			weeklyTasks: [flowTask('weekly-task')],
			source: 'sqlite',
			metrics: {
				totalTasks: 1,
				completedTasks: 1,
				requestedTasks: 0,
				pausedTasks: 0,
				stoppedTasks: 0,
				statusCounts: { 완료: 1 },
				businessCounts: { 사업: 1 },
				typeCounts: { 기타: 1 },
				memberScores: { 'member-1': 7 },
				memberScoreDetails: { 'member-1': { weeklyScore: 7, monthlyScore: 7, currentScore: 7 } },
				totalScore: 7
			}
		};

		const summary = mergeFlowSummary(state, weeklySummary);

		expect(summary.tasks).toEqual([flowTask('global-task')]);
		expect(summary.weeklyTasks).toEqual([flowTask('weekly-task')]);
		expect(summary.metrics.totalTasks).toBe(1);
		expect(summary.metrics.memberScores).toEqual({ 'member-1': 42 });
		expect(summary.metrics.memberScoreDetails?.['member-1'].currentScore).toBe(42);
		expect(summary.metrics.totalScore).toBe(42);
	});

});

function flowTask(id: string) {
	return {
		id,
		ownerID: 'member-1',
		ownerName: 'Member',
		participantIDs: ['member-1'],
		participantNames: ['Member'],
		business: '사업',
		type: '기타',
		content: 'Task',
		goal: '',
		size: 'S',
		status: '완료',
		weekCode: '26W23',
		flag: 0
	};
}
