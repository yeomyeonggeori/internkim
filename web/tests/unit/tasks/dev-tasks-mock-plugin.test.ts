import { describe, expect, test } from 'bun:test';
import { createDevTasksMockResponse, createDevTasksMockState } from '../../../dev-tasks-mock-plugin';
import type { TaskRunsResponse } from '../../../src/routes/tasks/tasks-api';

describe('dev tasks mock plugin', () => {
	test('returns task runs with limit and offset pagination', async () => {
		const state = createDevTasksMockState('admin@example.com');
		const response = await createDevTasksMockResponse(state, {
			method: 'GET',
			pathname: '/tasks/api/runs',
			searchParams: new URLSearchParams('limit=15&offset=15&includeTotal=true')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as TaskRunsResponse;
		expect(body.totalCount).toBe(60);
		expect(body.taskRuns.length).toBe(15);
		expect(body.taskRuns[0]?.taskRunID).toBe('dev-task-run-016');
	});

	test('keeps the admin diagnostics task list mock available', async () => {
		const state = createDevTasksMockState('admin@example.com');
		const response = await createDevTasksMockResponse(state, {
			method: 'GET',
			pathname: '/admin/api/diagnostics/tasks',
			searchParams: new URLSearchParams('limit=15&offset=15&includeTotal=true')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as TaskRunsResponse;
		expect(body.totalCount).toBe(60);
		expect(body.taskRuns[0]?.taskRunID).toBe('dev-task-run-016');
	});

	test('filters task runs by status before pagination', async () => {
		const state = createDevTasksMockState('admin@example.com');
		const response = await createDevTasksMockResponse(state, {
			method: 'GET',
			pathname: '/admin/api/diagnostics/tasks',
			searchParams: new URLSearchParams('status=failed&limit=15&offset=0&includeTotal=true')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as TaskRunsResponse;
		expect(body.taskRuns.length > 0).toBe(true);
		expect(body.taskRuns.every((taskRun) => taskRun.status === 'failed')).toBe(true);
	});

	test('returns an authenticated development session', async () => {
		const state = createDevTasksMockState('admin@example.com');
		const response = await createDevTasksMockResponse(state, {
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
