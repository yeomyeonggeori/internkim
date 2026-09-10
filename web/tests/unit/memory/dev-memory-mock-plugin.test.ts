import { describe, expect, test } from 'bun:test';
import { createDevMemoryMockResponse, createDevMemoryMockState } from '../../../dev-memory-mock-plugin';
import type { MemoryScheduleListResponse } from '../../../src/routes/memory/memory-schedule-api';
import type { MemoryFactsResponse } from '../../../src/routes/memory/memory-facts-api';

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

	test('returns facts and a profile so the memory tab can load, and forgets on request', async () => {
		const state = createDevMemoryMockState('admin@example.com');
		const response = await createDevMemoryMockResponse(state, {
			method: 'GET',
			pathname: '/memory/api/facts',
			searchParams: new URLSearchParams('limit=200')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as MemoryFactsResponse;
		expect(body.profile.identityLines.length > 0).toBe(true);
		expect(body.facts.length).toBe(5);

		const forgotten = await createDevMemoryMockResponse(state, {
			method: 'POST',
			pathname: '/memory/api/facts/forget',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({ factIDs: ['dev-fact-2'], reason: 'test' })
		});
		expect(forgotten?.status).toBe(200);
		const after = await createDevMemoryMockResponse(state, {
			method: 'GET',
			pathname: '/memory/api/facts',
			searchParams: new URLSearchParams()
		});
		expect((after?.body as MemoryFactsResponse).facts.map((fact) => fact.factID)).not.toContain('dev-fact-2');
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
