import { afterAll, describe, expect, mock, test } from 'bun:test';

const deviceRequests: Request[] = [];
const companyCalls: { capability: string; body: Record<string, unknown> }[] = [];
let centralConfigured = false;

const adminAPI = { ...(await import('../../../src/lib/admin-api')) };
const centralPlane = { ...(await import('../../../src/lib/supabase')) };
const hostBridge = { ...(await import('../../../src/lib/host-bridge')) };

mock.module('../../../src/lib/admin-api', () => ({
	adminApiFetch: async (path: string, options?: RequestInit) => {
		deviceRequests.push(new Request(`http://localhost${path}`, options));
		return new Response(JSON.stringify({ taskRunID: 'new-run', status: 'planned' }), { status: 200 });
	}
}));
mock.module('../../../src/lib/supabase', () => ({ isSupabaseConfigured: () => centralConfigured }));
mock.module('../../../src/lib/host-bridge', () => ({
	callCompanyApp: async (call: { capability: string; body: Record<string, unknown> }) => {
		companyCalls.push(call);
		return { status: 200, body: { taskRunID: 'company-run', status: 'running' } };
	}
}));

const { retryTaskRun } = await import('../../../src/routes/runs/runs-api');

afterAll(() => {
	mock.module('../../../src/lib/admin-api', () => adminAPI);
	mock.module('../../../src/lib/supabase', () => centralPlane);
	mock.module('../../../src/lib/host-bridge', () => hostBridge);
});

describe('retryTaskRun', () => {
	test('posts the source run ID to the device endpoint', async () => {
		const response = await retryTaskRun('failed-run');
		const request = deviceRequests.at(-1);

		expect(response).toEqual({ taskRunID: 'new-run', status: 'planned' });
		if (!request) throw new Error('device retry request was not recorded');
		expect(request.url).toBe('http://localhost/runs/api/retry');
		expect(await request.text()).toBe(JSON.stringify({ taskRunID: 'failed-run' }));
	});

	test('uses the central retry capability when configured', async () => {
		centralConfigured = true;
		const response = await retryTaskRun('failed-company-run');

		expect(response).toEqual({ taskRunID: 'company-run', status: 'running' });
		expect(companyCalls.at(-1)).toEqual({ capability: 'person.runs.retry', body: { taskRunID: 'failed-company-run' } });
		centralConfigured = false;
	});
});
