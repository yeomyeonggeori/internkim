import { describe, expect, test } from 'bun:test';
import { normalizeMemoryScheduleListResponse } from '../../../src/routes/memory/memory-schedule-api';

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
			checkedAt: '2026-06-08T00:00:00Z'
		});

		expect(response.schedules?.length).toBe(1);
		expect(response.schedules?.[0]?.taskScheduleID).toBe('schedule-1');
		expect(response.schedules?.[0]?.executionMode).toBe('agent');
		expect(response.schedules?.[0]?.kind).toBe('cron');
		expect(response.schedules?.[0]?.cronExpression).toBe('0 9 * * *');
		expect(response.schedules?.[0]?.intervalSecond).toBe(undefined);
		expect(response.count).toBe(2);
	});
});
