import { describe, expect, test } from 'bun:test';
import {
	cancelSchedule,
	fetchMemorySchedules,
	normalizeMemoryScheduleListResponse,
	updateSchedule
} from '../../../src/routes/memory/memory-schedule-api';

describe('memory schedule api normalizer', () => {
	test('drops malformed schedule rows while keeping valid schedules', () => {
		const response = normalizeMemoryScheduleListResponse({
			schedules: [
				{
					taskScheduleID: 'schedule-1',
					executionMode: 'agent',
					kind: 'cron',
					cronExpression: '0 9 * * *',
					nextRunAt: '2026-06-09T00:00:00Z',
					intervalSecond: 'not-a-number'
				},
				{
					taskScheduleID: 123,
					executionMode: 'agent',
					kind: 'cron'
				}
			],
			count: 2,
			totalCount: 70,
			page: 3,
			pageSize: 25,
			checkedAt: '2026-06-08T00:00:00Z'
		});

		expect(response.schedules?.length).toBe(1);
		expect(response.schedules?.[0]?.taskScheduleID).toBe('schedule-1');
		expect(response.schedules?.[0]?.executionMode).toBe('agent');
		expect(response.schedules?.[0]?.kind).toBe('cron');
		expect(response.schedules?.[0]?.cronExpression).toBe('0 9 * * *');
		expect(response.schedules?.[0]?.intervalSecond).toBe(undefined);
		expect(response.count).toBe(2);
		expect(response.totalCount).toBe(70);
		expect(response.page).toBe(3);
		expect(response.pageSize).toBe(25);
	});

	test('sends pagination parameters when fetching schedules', async () => {
		const originalFetch = globalThis.fetch;
		let requestedURL = '';
		const fetchStub = createFetchStub(async (input) => {
			requestedURL = String(input);
			return new Response(JSON.stringify({ schedules: [], count: 0, totalCount: 0, page: 3, pageSize: 25 }));
		});
		globalThis.fetch = fetchStub;

		try {
			const response = await fetchMemorySchedules({ page: 3, pageSize: 25 });

			expect(requestedURL).toBe('/memory/api/schedules?page=3&pageSize=25');
			expect(response.page).toBe(3);
			expect(response.pageSize).toBe(25);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('does not expose schedule list failure response text', async () => {
		const originalFetch = globalThis.fetch;
		const fetchStub = createFetchStub(async () => new Response('Traceback /workspace/.blueclaw/private.py', { status: 502 }));
		globalThis.fetch = fetchStub;

		try {
			const errorMessage = await rejectedErrorMessage(fetchMemorySchedules());

			expect(errorMessage).toBe('Memory schedules request returned 502');
			expect(errorMessage.includes('Traceback')).toBe(false);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('posts schedule cancellation request', async () => {
		const originalFetch = globalThis.fetch;
		let requestedURL = '';
		let requestedInitialization: Parameters<typeof fetch>[1] | undefined;
		const fetchStub = createFetchStub(async (input, initialization) => {
			requestedURL = String(input);
			requestedInitialization = initialization;
			return new Response('{}');
		});
		globalThis.fetch = fetchStub;

		try {
			await cancelSchedule('schedule-1');

			expect(requestedURL).toBe('/memory/api/schedules/cancel');
			expect(requestedInitialization?.method).toBe('POST');
			expect(requestedInitialization?.credentials).toBe('include');
			expect(requestedInitialization?.body).toBe(JSON.stringify({ taskScheduleID: 'schedule-1' }));
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('posts schedule update request fields', async () => {
		const originalFetch = globalThis.fetch;
		let requestedURL = '';
		let requestedInitialization: Parameters<typeof fetch>[1] | undefined;
		const fetchStub = createFetchStub(async (input, initialization) => {
			requestedURL = String(input);
			requestedInitialization = initialization;
			return new Response('{}');
		});
		globalThis.fetch = fetchStub;

		try {
			await updateSchedule('schedule-1', {
				name: 'Daily reminder',
				kind: 'interval',
				intervalSecond: 1800,
				repeatPolicy: 'unbounded'
			});

			expect(requestedURL).toBe('/memory/api/schedules/update');
			expect(requestedInitialization?.method).toBe('POST');
			expect(requestedInitialization?.credentials).toBe('include');
			expect(requestedInitialization?.body).toBe(
				JSON.stringify({
					taskScheduleID: 'schedule-1',
					name: 'Daily reminder',
					kind: 'interval',
					intervalSecond: 1800,
					repeatPolicy: 'unbounded'
				})
			);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('does not expose schedule mutation failure response text', async () => {
		const originalFetch = globalThis.fetch;
		const fetchStub = createFetchStub(async () => new Response('Graphiti stack trace', { status: 502 }));
		globalThis.fetch = fetchStub;

		try {
			const cancelErrorMessage = await rejectedErrorMessage(cancelSchedule('schedule-1'));
			const updateErrorMessage = await rejectedErrorMessage(updateSchedule('schedule-1', { kind: 'once', runAt: '2026-06-12T00:00:00Z' }));

			expect(cancelErrorMessage).toBe('Memory schedules request returned 502');
			expect(updateErrorMessage).toBe('Memory schedules request returned 502');
			expect(cancelErrorMessage.includes('Graphiti')).toBe(false);
			expect(updateErrorMessage.includes('Graphiti')).toBe(false);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});

async function rejectedErrorMessage(promise: Promise<unknown>): Promise<string> {
	try {
		await promise;
		return '';
	} catch (error) {
		return error instanceof Error ? error.message : '';
	}
}

function createFetchStub(
	handler: (input: Parameters<typeof fetch>[0], initialization?: Parameters<typeof fetch>[1]) => Promise<Response>
) {
	return Object.assign(handler, { preconnect: (url: string | URL): void => void url });
}
