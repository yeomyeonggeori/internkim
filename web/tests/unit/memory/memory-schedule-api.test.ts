import { describe, expect, test } from 'bun:test';
import {
	fetchMemorySchedules,
	normalizeMemoryScheduleListResponse
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
			checkedAt: '2026-06-08T00:00:00Z',
			currentPersonID: 'person-1'
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
		expect(response.currentPersonID).toBe('person-1');
	});

	test('sends pagination parameters when fetching schedules', async () => {
		const originalFetch = globalThis.fetch;
		let requestedURL = '';
		globalThis.fetch = (async (input: RequestInfo | URL) => {
			requestedURL = String(input);
			return new Response(JSON.stringify({ schedules: [], count: 0, totalCount: 0, page: 3, pageSize: 25 }));
		}) as typeof fetch;

		try {
			const response = await fetchMemorySchedules({ page: 3, pageSize: 25 });

			expect(requestedURL).toBe('/memory/api/schedules?page=3&pageSize=25');
			expect(response.page).toBe(3);
			expect(response.pageSize).toBe(25);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});
