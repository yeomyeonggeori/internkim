import { describe, expect, test } from 'bun:test';
import { parseSupabaseWorkPolicies } from '../../../src/lib/attendance/supabase-work-policy';

describe('parseSupabaseWorkPolicies', () => {
	test('preserves the company work mode returned for each member', () => {
		const policies = parseSupabaseWorkPolicies([
			{
				member_id: 'member-fixed',
				work_hours: null,
				minimum_daily_minutes: 480,
				work_mode: 'fixed'
			},
			{
				member_id: 'member-autonomous',
				work_hours: [[null, null, null, null, null, null, null]],
				minimum_daily_minutes: null,
				work_mode: 'autonomous'
			}
		]);

		expect(policies.get('member-fixed')?.workMode).toBe('fixed');
		expect(policies.get('member-autonomous')?.workMode).toBe('autonomous');
	});

	test('rejects a work mode outside the supported company policy', () => {
		expect(() =>
			parseSupabaseWorkPolicies([
				{
					member_id: 'member-hybrid',
					work_hours: null,
					minimum_daily_minutes: 480,
					work_mode: 'hybrid'
				}
			])
		).toThrow('work mode is invalid for member member-hybrid');
	});
});
