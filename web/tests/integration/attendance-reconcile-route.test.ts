import { afterAll, beforeAll, beforeEach, describe, expect, mock, test } from 'bun:test';
import {
	controlPlane,
	issueAgentKey,
	provisionCompany
} from '../../src/lib/server/control-plane';

mock.module('$env/dynamic/private', () => ({ env: {} }));

type AttendanceReconcileHandler = (typeof import('../../src/routes/api/agent/attendance-reconcile/+server'))['POST'];
type AttendanceReconcileEvent = Parameters<AttendanceReconcileHandler>[0];

const projectURL = process.env.SUPABASE_URL ?? '';
const serviceRoleKey = process.env.SUPABASE_SERVICE_ROLE_KEY ?? '';
const canReachSupabase = Boolean(projectURL && serviceRoleKey);
const client = canReachSupabase ? controlPlane({ projectURL, serviceRoleKey }) : null;
const stamp = Date.now();
let companyID = '';
let agentKey = '';

beforeAll(async () => {
	if (!client) return;
	const company = await provisionCompany(
		client,
		{
			name: 'Attendance Reconcile Route',
			slug: `attendance-reconcile-${stamp}`,
			country: 'KR',
			locale: 'ko',
			timezone: 'Asia/Seoul'
		},
		`attendance-reconcile-${stamp}@example.test`
	);
	companyID = company.companyID;
	const issued = await issueAgentKey(client, companyID, 'route-test');
	agentKey = issued.apiKey;
});

beforeEach(async () => {
	if (!client || !companyID) return;
	const { error } = await client
		.from('company')
		.update({ rules: { approvals: { required: true } } })
		.eq('id', companyID);
	if (error) throw new Error(error.message);
});

afterAll(async () => {
	if (!client || !companyID) return;
	await client.from('company').delete().eq('id', companyID);
});

if (!canReachSupabase) {
	test('supabase is not reachable, so the attendance reconciliation route is not exercised', () => {
		expect(canReachSupabase).toBe(false);
	});
}

if (canReachSupabase) {
	describe('attendance reconciliation route', () => {
		test('a legacy request reconciles without changing company rules', async () => {
			const response = await reconcile({
				platform: 'mattermost',
				from: '2027-01-01T00:00:00Z',
				to: '2027-01-02T00:00:00Z',
				events: []
			});

			expect(response.status).toBe(200);
			expect(await companyRules()).toEqual({ approvals: { required: true } });
		});

		test('a valid work calendar is persisted without replacing unrelated rules', async () => {
			const response = await reconcile({
				platform: 'mattermost',
				workMode: 'fixed',
				workCalendar: [
					{ date: '2027-01-01', workMode: 'fixed', workingDate: false },
					{ date: '2027-01-02', workMode: 'fixed', workingDate: true }
				],
				from: '2027-01-01T00:00:00Z',
				to: '2027-01-03T00:00:00Z',
				events: []
			});

			expect(response.status).toBe(200);
			expect(await companyRules()).toEqual({
				approvals: { required: true },
				attendanceCalendar: [
					{ date: '2027-01-01', workMode: 'fixed', workingDate: false },
					{ date: '2027-01-02', workMode: 'fixed', workingDate: true }
				]
			});
		});

		test('an explicitly malformed work calendar returns 400', async () => {
			await expect(
				reconcile({
					platform: 'mattermost',
					workMode: 'fixed',
					workCalendar: [{ date: '2027-01-01', workMode: 'fixed' }],
					from: '2027-01-01T00:00:00Z',
					to: '2027-01-02T00:00:00Z',
					events: []
				})
			).rejects.toMatchObject({ status: 400 });
		});
	});
}

async function reconcile(payload: Record<string, unknown>): Promise<Response> {
	const { POST } = await import('../../src/routes/api/agent/attendance-reconcile/+server');
	const request = new Request('https://api.example.test/api/agent/attendance-reconcile', {
		method: 'POST',
		headers: {
			authorization: `Bearer ${agentKey}`,
			'content-type': 'application/json'
		},
		body: JSON.stringify(payload)
	});
	return POST({
		request,
		platform: { env: { SUPABASE_URL: projectURL, SUPABASE_SERVICE_ROLE_KEY: serviceRoleKey } }
	} as unknown as AttendanceReconcileEvent);
}

async function companyRules(): Promise<unknown> {
	if (!client) throw new Error('the integration database is not configured');
	const { data, error } = await client.from('company').select('rules').eq('id', companyID).single();
	if (error) throw new Error(error.message);
	return data.rules;
}
