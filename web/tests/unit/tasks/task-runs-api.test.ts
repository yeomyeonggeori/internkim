import { describe, expect, test } from 'bun:test';
import { taskRunsAPIPath } from '../../../src/routes/runs/runs-api';

describe('taskRunsAPIPath', () => {
	test('requests task runs with status, limit, offset, and total count pagination', () => {
		const path = taskRunsAPIPath({ status: 'failed', limit: 15, offset: 15, includeTotal: true, includeCost: true, dailyCostTaskRunLimit: 500 });

		expect(path).toBe('/runs/api?limit=15&offset=15&dailyCostTaskRunLimit=500&includeTotal=true&includeCost=true&status=failed');
	});
});
