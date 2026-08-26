import { describe, expect, test } from 'bun:test';
import { createDevTasksMockResponse, createDevTasksMockState } from '../../../dev-tasks-mock-plugin';
import type { ServiceLogsResponse, TaskDetail, TaskRunsResponse } from '../../../src/routes/runs/runs-api';

describe('dev tasks mock plugin', () => {
	test('returns task runs with limit and offset pagination', async () => {
		const state = createDevTasksMockState('admin@example.com');
		const response = await createDevTasksMockResponse(state, {
			method: 'GET',
			pathname: '/tasks/api/runs',
			searchParams: new URLSearchParams('limit=15&offset=15&includeTotal=true&includeCost=true&dailyCostTaskRunLimit=10')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as TaskRunsResponse;
		expect(body.totalCount).toBe(60);
		expect(body.taskRuns.length).toBe(15);
		expect(body.taskRuns[0]?.taskRunID).toBe('dev-task-run-016');
		expect((body.dailyCostSummaries?.length ?? 0) > 0).toBe(true);
		expect((body.dailyCostSummaries?.[0]?.costUSD ?? 0) > 0).toBe(true);
		expect((body.taskRuns[0]?.llmCostUSD ?? 0) > 0).toBe(true);
		expect(body.dailyCostScope?.taskRunCount).toBe(10);
		expect(body.dailyCostScope?.isTruncated).toBe(true);
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

	test('returns task detail with timeline events', async () => {
		const state = createDevTasksMockState('admin@example.com');
		const response = await createDevTasksMockResponse(state, {
			method: 'GET',
			pathname: '/tasks/api/run-detail',
			searchParams: new URLSearchParams('taskRunID=dev-task-run-001')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as TaskDetail;
		expect(body.taskRun.taskRunID).toBe('dev-task-run-001');
		expect(body.taskEvents.some((taskEvent) => taskEvent.name === 'llm.call')).toBe(true);
		expect(body.taskEvents.some((taskEvent) => taskEvent.name === 'tool.site.app.publish.result')).toBe(true);
	});

	test('deletes terminal task runs from the mock list', async () => {
		const state = createDevTasksMockState('admin@example.com');
		const deleteResponse = await createDevTasksMockResponse(state, {
			method: 'DELETE',
			pathname: '/tasks/api/runs/dev-task-run-001',
			searchParams: new URLSearchParams()
		});
		const listResponse = await createDevTasksMockResponse(state, {
			method: 'GET',
			pathname: '/tasks/api/runs',
			searchParams: new URLSearchParams('limit=15&offset=0&includeTotal=true')
		});

		expect(deleteResponse?.status).toBe(200);
		const body = listResponse?.body as TaskRunsResponse;
		expect(body.totalCount).toBe(59);
		expect(body.taskRuns.some((taskRun) => taskRun.taskRunID === 'dev-task-run-001')).toBe(false);
	});

	test('returns correlated service logs', async () => {
		const state = createDevTasksMockState('admin@example.com');
		const response = await createDevTasksMockResponse(state, {
			method: 'GET',
			pathname: '/admin/api/diagnostics/service-logs',
			searchParams: new URLSearchParams('service=blueclaw&taskRunID=dev-task-run-001')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as ServiceLogsResponse;
		expect(body.taskRunID).toBe('dev-task-run-001');
		expect(body.lines.some((line) => line.includes('publish URL'))).toBe(true);
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
