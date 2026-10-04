import { expect, mock, test } from 'bun:test';
import { defaultLeavePolicy } from '../../../src/lib/attendance/leave-policy-defaults';

const asked: { name: string; input: Record<string, unknown> }[] = [];
let releasePolicy: (() => void) | undefined;
const policyGate = new Promise<void>((resolve) => { releasePolicy = resolve; });

mock.module('../../../src/lib/public-api-call', () => ({
	ToolRefused: class extends Error {},
	invokeTool: async (name: string, input: Record<string, unknown>) => {
		asked.push({ name, input });
		if (name === 'attendance_leave_policy_get') { await policyGate; return defaultLeavePolicy(); }
		if (name === 'leave_balance') return {
			scope: 'mine', year: 2026, count: 1, balances: [{
				personID: 'member-one', personName: '이샘플', grantedDays: 15, remainingDays: 15, usedDays: 0, tracking: 'managed'
			}]
		};
		if (name === 'leave_list') return { count: 0, leave: [], registeredKinds: [] };
		if (name === 'company_settings_get') return { timeZone: 'Asia/Seoul' };
		if (name === 'person_list') return {
			requesterID: 'member-one', count: 1,
			people: [{ personID: 'member-one', name: '이샘플', email: 'sample@example.com' }]
		};
		throw new Error(`Unexpected tool ${name}`);
	}
}));

const { supabaseEmployeeLeave } = await import('../../../src/lib/attendance/supabase-leave');

test('employee leave starts independent reads together and requests only the caller’s leave', async () => {
	const reading = supabaseEmployeeLeave();
	expect(asked.map((call) => call.name)).toEqual([
		'attendance_leave_policy_get', 'leave_balance', 'leave_list', 'company_settings_get', 'person_list'
	]);
	expect(asked.find((call) => call.name === 'leave_list')?.input).toEqual({});
	if (!releasePolicy) throw new Error('policy gate was not initialized');
	releasePolicy();
	const payload = await reading;
	expect(payload.summary.availableMilliDays).toBe(15000);
	expect(payload.requests).toEqual([]);
});
