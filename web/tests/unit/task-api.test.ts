import { describe, expect, test } from 'bun:test';
import {
	createQuickTask,
	deleteTask,
	fetchTaskState,
	fetchTaskWeeklySummary,
	mergeTaskSummary,
	moveTaskOnBoard,
	saveTask,
	updateTaskParent,
	updateTaskParents
} from '../../src/routes/task/task-api';

type FetchWithPreconnect = typeof fetch & { preconnect?: unknown };

function fetchPreconnect(fetchValue: typeof fetch) {
	return (fetchValue as FetchWithPreconnect).preconnect;
}

describe('flow API', () => {
	test('updates multiple parent relationships with one request', async () => {
		const originalFetch = globalThis.fetch;
		let requestedURL = '';
		let requestBody: unknown = null;

		try {
			globalThis.fetch = Object.assign(
				async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
					requestedURL = String(input);
					requestBody = JSON.parse(String(init?.body ?? '{}'));
					return new Response(null, { status: 204 });
				},
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			await updateTaskParents(['child-1', 'child-2'], 'parent', 'Could not update relationships.');

			expect(requestedURL).toBe('/task/api/tasks/parents');
			expect(requestBody).toEqual({ taskIDs: ['child-1', 'child-2'], parentTaskID: 'parent' });
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

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

			const result = await createQuickTask(
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

			const result = await createQuickTask(
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
				createQuickTask(
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

	test('omits local create-only fields when creating a task so the server can append it', async () => {
		const originalFetch = globalThis.fetch;
		let requestBody: Record<string, unknown> = {};

		try {
			globalThis.fetch = Object.assign(
				async (_input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
					requestBody = JSON.parse(String(init?.body ?? '{}')) as Record<string, unknown>;
					return new Response(null, { status: 204 });
				},
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			await saveTask(taskOf(''), 'Could not save the task.', null);

			expect('id' in requestBody).toBe(false);
			expect('statusRank' in requestBody).toBe(false);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('keeps explicit create status rank without sending a blank task id', async () => {
		const originalFetch = globalThis.fetch;
		let requestBody: Record<string, unknown> = {};

		try {
			globalThis.fetch = Object.assign(
				async (_input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
					requestBody = JSON.parse(String(init?.body ?? '{}')) as Record<string, unknown>;
					return new Response(null, { status: 204 });
				},
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			await saveTask({ ...taskOf(''), statusRank: 2048 }, 'Could not save the task.', null);

			expect('id' in requestBody).toBe(false);
			expect(requestBody.statusRank).toBe(2048);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('omits read-only createdAt when saving an existing task', async () => {
		const originalFetch = globalThis.fetch;
		let requestBody: Record<string, unknown> = {};

		try {
			globalThis.fetch = Object.assign(
				async (_input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
					requestBody = JSON.parse(String(init?.body ?? '{}')) as Record<string, unknown>;
					return new Response(null, { status: 204 });
				},
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			await saveTask({
				...taskOf('task-1'),
				parentTaskID: 'stale-parent',
				createdAt: '2026-06-01T10:00:00Z'
			}, 'Could not save the task.', null);

			expect('createdAt' in requestBody).toBe(false);
			expect('parentTaskID' in requestBody).toBe(false);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('posts board move requests to the atomic move endpoint', async () => {
		const originalFetch = globalThis.fetch;
		let requestedURL = '';
		let requestMethod = '';
		let requestBody: Record<string, unknown> = {};

		try {
			globalThis.fetch = Object.assign(
				async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
					requestedURL = String(input);
					requestMethod = init?.method ?? '';
					requestBody = JSON.parse(String(init?.body ?? '{}')) as Record<string, unknown>;
					return new Response(null, { status: 204 });
				},
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			await moveTaskOnBoard({
				taskID: 'task-1',
				targetStatus: 'in_progress',
				beforeTaskID: 'task-2'
			}, 'Could not save the task.');

			expect(requestedURL).toBe('/task/api/tasks/move');
			expect(requestMethod).toBe('POST');
			expect(requestBody).toEqual({
				taskID: 'task-1',
				targetStatus: 'in_progress',
				beforeTaskID: 'task-2'
			});
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('deletes a task through the task endpoint', async () => {
		const originalFetch = globalThis.fetch;
		let requestedURL = '';
		let requestMethod = '';

		try {
			globalThis.fetch = Object.assign(
				async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
					requestedURL = String(input);
					requestMethod = init?.method ?? '';
					return new Response(null, { status: 204 });
				},
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			await deleteTask('task-1', 'Could not delete the task.');

			expect(requestedURL).toBe('/task/api/tasks/task-1');
			expect(requestMethod).toBe('DELETE');
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('updates only the parent relationship through the task parent endpoint', async () => {
		const originalFetch = globalThis.fetch;
		const requests: Array<{ url: string; method: string; body: unknown }> = [];

		try {
			globalThis.fetch = Object.assign(
				async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
					requests.push({
						url: String(input),
						method: init?.method ?? '',
						body: JSON.parse(String(init?.body ?? '{}'))
					});
					return new Response(null, { status: 204 });
				},
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			await updateTaskParent('child/task', 'parent-1', 'Could not update the task relationship.');
			await updateTaskParent('child/task', undefined, 'Could not update the task relationship.');

			expect(requests).toEqual([
				{
					url: '/task/api/tasks/child%2Ftask/parent',
					method: 'PATCH',
					body: { parentTaskID: 'parent-1' }
				},
				{
					url: '/task/api/tasks/child%2Ftask/parent',
					method: 'PATCH',
					body: { parentTaskID: null }
				}
			]);
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

			await fetchTaskWeeklySummary('', 'Could not load Flow data.');

			expect(requestedURL).toBe('/task/api/summary');
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
				{ preconnect: fetchPreconnect(originalFetch) }
			);

			await fetchTaskState('Could not load Flow data.');

			expect(requestedURL).toBe('/task/api/state');
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
			tasks: [taskOf('global-task')],
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
			weeklyTasks: [taskOf('weekly-task')],
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

		const summary = mergeTaskSummary(state, weeklySummary);

		expect(summary.tasks).toEqual([taskOf('global-task')]);
		expect(summary.weeklyTasks).toEqual([taskOf('weekly-task')]);
		expect(summary.metrics.totalTasks).toBe(1);
		expect(summary.metrics.memberScores).toEqual({ 'member-1': 42 });
		expect(summary.metrics.memberScoreDetails?.['member-1'].currentScore).toBe(42);
		expect(summary.metrics.totalScore).toBe(42);
	});

});

function taskOf(id: string) {
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
		status: 'completed',
		statusRank: 0,
		weekCode: '26W23',
		flag: 0
	};
}
