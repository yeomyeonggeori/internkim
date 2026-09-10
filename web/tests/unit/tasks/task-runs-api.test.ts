import { describe, expect, test } from 'bun:test';
import { readRetryTaskRunResponse, taskRunsAPIPath } from '../../../src/routes/runs/runs-api';

describe('taskRunsAPIPath', () => {
	test('requests task runs with status, limit, offset, and total count pagination', () => {
		const path = taskRunsAPIPath({ status: 'failed', limit: 15, offset: 15, includeTotal: true, includeCost: true, dailyCostTaskRunLimit: 500 });

		expect(path).toBe('/runs/api?limit=15&offset=15&dailyCostTaskRunLimit=500&includeTotal=true&includeCost=true&status=failed');
	});
});

describe('readRetryTaskRunResponse', () => {
	test('accepts the new run identity and status', () => {
		expect(readRetryTaskRunResponse({ taskRunID: 'new-run', status: 'planned' })).toEqual({ taskRunID: 'new-run', status: 'planned' });
	});

	test('rejects a response without a usable run identity or status', () => {
		expect(() => readRetryTaskRunResponse({ taskRunID: '', status: 'planned' })).toThrow('malformed');
		expect(() => readRetryTaskRunResponse({ taskRunID: 'new-run' })).toThrow('malformed');
	});
});
