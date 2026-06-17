import { describe, expect, test } from 'bun:test';
import { createDevMemoryMockResponse, createDevMemoryMockState } from '../../../dev-memory-mock-plugin';
import type { MemoryScheduleListResponse } from '../../../src/routes/memory/memory-schedule-api';
import type { MemoryGraphResponse } from '../../../src/routes/memory/memory-graph-api';

describe('dev memory mock plugin', () => {
	test('returns schedules with page and page size pagination', async () => {
		const state = createDevMemoryMockState('admin@example.com');
		const response = await createDevMemoryMockResponse(state, {
			method: 'GET',
			pathname: '/memory/api/schedules',
			searchParams: new URLSearchParams('page=2&pageSize=15&includeExpired=true')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as MemoryScheduleListResponse;
		expect(body.totalCount).toBe(58);
		expect(body.page).toBe(2);
		expect(body.pageSize).toBe(15);
		expect(body.schedules?.length).toBe(15);
		expect(body.schedules?.[0]?.taskScheduleID).toBe('dev-schedule-016');
	});

	test('returns a graph payload so the memory tab can load', async () => {
		const state = createDevMemoryMockState('admin@example.com');
		const response = await createDevMemoryMockResponse(state, {
			method: 'GET',
			pathname: '/memory/api/graph',
			searchParams: new URLSearchParams('limit=120')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as MemoryGraphResponse;
		expect(body.health).toMatchObject({ configured: true, reachable: true });
		expect((body.nodes?.length ?? 0) > 0).toBe(true);
	});

	test('returns an authenticated development session', async () => {
		const state = createDevMemoryMockState('admin@example.com');
		const response = await createDevMemoryMockResponse(state, {
			method: 'GET',
			pathname: '/auth/session',
			searchParams: new URLSearchParams()
		});

		expect(response).toEqual({
			status: 200,
			body: { authenticated: true, email: 'admin@example.com', isAdmin: true }
		});
	});
});
