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
					{ date: '2027-01-01', workMode: 'fixed', workingDate: false, holiday: true },
					{ date: '2027-01-02', workMode: 'fixed', workingDate: true, holiday: false }
				],
				from: '2027-01-01T00:00:00Z',
				to: '2027-01-03T00:00:00Z',
				events: []
			});

			expect(response.status).toBe(200);
			expect(await companyRules()).toEqual({
				approvals: { required: true },
				attendanceCalendar: [
					{ date: '2027-01-01', workMode: 'fixed', workingDate: false, holiday: true },
					{ date: '2027-01-02', workMode: 'fixed', workingDate: true, holiday: false }
				]
			});
		});

		test('current policy and calendar are persisted together without replacing unrelated rules', async () => {
			const workPolicy = currentPolicy();
			const workCalendar = [
				{ date: '2027-01-01', workMode: 'fixed', workingDate: false, holiday: true }
			];
			const response = await reconcile({
				platform: 'mattermost',
				workPolicy,
				workCalendar,
				from: '2027-01-01T00:00:00Z',
				to: '2027-01-02T00:00:00Z',
				events: []
			});

			expect(response.status).toBe(200);
			expect(await companyRules()).toEqual({
				approvals: { required: true },
				attendanceCalendar: workCalendar,
				attendanceWorkPolicy: workPolicy
			});
		});

		test('an invalid platform returns 400 before settings persistence', async () => {
			await expect(
				reconcile({
					platform: '',
					workPolicy: currentPolicy(),
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

		test('a window over one month returns 400 before date enumeration or persistence', async () => {
			const originalSetUTCDate = Date.prototype.setUTCDate;
			Date.prototype.setUTCDate = function (): number {
				throw new Error('calendar date enumeration started');
			};
			try {
				await expect(
					reconcile({
						platform: 'mattermost',
						workMode: 'fixed',
						workCalendar: [],
						from: '0100-01-01T00:00:00Z',
						to: '9999-01-01T00:00:00Z',
						events: []
					})
				).rejects.toMatchObject({ status: 400 });
				expect(await companyRules()).toEqual({ approvals: { required: true } });
			} finally {
				Date.prototype.setUTCDate = originalSetUTCDate;
			}
		});

		const completeWindow = [
			{ date: '2027-01-01', workMode: 'fixed', workingDate: true },
			{ date: '2027-01-02', workMode: 'fixed', workingDate: true },
			{ date: '2027-01-03', workMode: 'fixed', workingDate: false }
		];
		const invalidCalendars = [
			{ name: 'empty', workCalendar: [] },
			{ name: 'duplicate', workCalendar: [completeWindow[0], completeWindow[0], completeWindow[2]] },
			{ name: 'missing', workCalendar: [completeWindow[0], completeWindow[2]] },
			{ name: 'reversed', workCalendar: [completeWindow[1], completeWindow[0], completeWindow[2]] },
			{
				name: 'out-of-window',
				workCalendar: [
					{ date: '2026-12-31', workMode: 'fixed', workingDate: true },
					completeWindow[0],
					completeWindow[1]
				]
			},
			{
				name: 'oversized',
				workCalendar: [
					...completeWindow,
					{ date: '2027-01-04', workMode: 'fixed', workingDate: true }
				]
			}
		];
		for (const invalid of invalidCalendars) {
			test(`an explicitly ${invalid.name} work calendar returns 400 before persistence`, async () => {
				await expect(
					reconcile({
						platform: 'mattermost',
						workMode: 'fixed',
						workCalendar: invalid.workCalendar,
						from: '2027-01-01T00:00:00Z',
						to: '2027-01-04T00:00:00Z',
						events: []
					})
				).rejects.toMatchObject({ status: 400 });
				expect(await companyRules()).toEqual({ approvals: { required: true } });
			});
		}
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
