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
const serviceRoleKey = process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '';
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
			name: 'Sample Attendance Reconcile',
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

		test('a work policy is stored as the revision that has always applied', async () => {
			const workPolicy = currentPolicy();
			const response = await reconcile({
				platform: 'mattermost',
				workPolicy,
				from: '2027-01-01T00:00:00Z',
				to: '2027-01-02T00:00:00Z',
				events: []
			});

			expect(response.status).toBe(200);
			expect(await companyRules()).toEqual({
				approvals: { required: true },
				attendanceWorkPolicy: {
					version: 1,
					revisions: [{ ...workPolicy, effectiveDate: '1970-01-01' }]
				}
			});
		});

		test('a work calendar from an older device is ignored rather than stored', async () => {
			const workPolicy = currentPolicy();
			const response = await reconcile({
				platform: 'mattermost',
				workPolicy,
				workCalendar: [
					{ date: '2027-01-01', workMode: 'fixed', workingDate: false, holiday: true },
					{ date: '2027-01-02', workMode: 'fixed', workingDate: true, holiday: false }
				],
				from: '2027-01-01T00:00:00Z',
				to: '2027-01-03T00:00:00Z',
				events: []
			});

			expect(response.status).toBe(200);
			const rules = (await companyRules()) as Record<string, unknown>;
			expect(Object.keys(rules)).not.toContain('attendanceCalendar');
			expect(rules.attendanceWorkPolicy).toEqual({
				version: 1,
				revisions: [{ ...workPolicy, effectiveDate: '1970-01-01' }]
			});
		});

		test('a malformed work calendar from an older device is ignored too', async () => {
			const response = await reconcile({
				platform: 'mattermost',
				workMode: 'fixed',
				workCalendar: [{ date: '2027-01-01', workMode: 'fixed' }],
				from: '2027-01-01T00:00:00Z',
				to: '2027-01-04T00:00:00Z',
				events: []
			});

			expect(response.status).toBe(200);
			expect(await companyRules()).toEqual({ approvals: { required: true } });
		});

		test('the company holidays a device holds are carried over once', async () => {
			const holiday = {
				id: 'company-holiday-founding',
				title: '창립기념일',
				date: '2027-03-02',
				recursAnnually: true
			};
			const response = await reconcile({
				platform: 'mattermost',
				companyHolidays: [holiday],
				from: '2027-01-01T00:00:00Z',
				to: '2027-01-02T00:00:00Z',
				events: []
			});

			expect(response.status).toBe(200);
			const rules = (await companyRules()) as Record<string, unknown>;
			expect(rules.companyHolidays).toEqual([holiday]);

			await reconcile({
				platform: 'mattermost',
				companyHolidays: [holiday],
				from: '2027-01-01T00:00:00Z',
				to: '2027-01-02T00:00:00Z',
				events: []
			});
			expect(((await companyRules()) as Record<string, unknown>).companyHolidays).toEqual([holiday]);
		});

		test('a malformed company holiday returns 400 before persistence', async () => {
			await expect(
				reconcile({
					platform: 'mattermost',
					companyHolidays: [{ id: 'x', title: '', date: '2027-03-02', recursAnnually: true }],
					from: '2027-01-01T00:00:00Z',
					to: '2027-01-02T00:00:00Z',
					events: []
				})
			).rejects.toMatchObject({ status: 400 });
			expect(await companyRules()).toEqual({ approvals: { required: true } });
		});

		test('a malformed current work policy returns 400 before persistence', async () => {
			await expect(
				reconcile({
					platform: 'mattermost',
					workPolicy: { ...currentPolicy(), workMode: 'hybrid' },
					from: '2027-01-01T00:00:00Z',
					to: '2027-01-02T00:00:00Z',
					events: []
				})
			).rejects.toMatchObject({ status: 400 });
			expect(await companyRules()).toEqual({ approvals: { required: true } });
		});

		test('an invalid current policy time returns 400 before persistence', async () => {
			await expect(
				reconcile({
					platform: 'mattermost',
					workPolicy: { ...currentPolicy(), nightStartTime: '99:99' },
					from: '2027-01-01T00:00:00Z',
					to: '2027-01-02T00:00:00Z',
					events: []
				})
			).rejects.toMatchObject({ status: 400 });
			expect(await companyRules()).toEqual({ approvals: { required: true } });
		});

	});
}

function currentPolicy() {
	return {
		workMode: 'fixed',
		workingWeekdays: [1, 2, 3, 4, 5],
		dailyTargetMinutes: 480,
		weeklyTargetMinutes: 2400,
		referenceStartTime: '09:00',
		fixedStartTime: '09:00',
		fixedEndTime: '18:00',
		coreTimeEnabled: false,
		coreStartTime: '',
		coreEndTime: '',
		breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
		nightStartTime: '22:00',
		nightEndTime: '06:00'
	};
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
