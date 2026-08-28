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

	test('gives a member with no stored policy the one revision that has always applied', () => {
		const policies = parseSupabaseWorkPolicies([
			{
				member_id: 'member-legacy',
				work_hours: [[[], [], [], [], [], null, null]],
				minimum_daily_minutes: 480,
				work_mode: 'flexible'
			}
		]);

		const legacy = policies.get('member-legacy');
		expect(legacy?.revisions).toHaveLength(1);
		expect(legacy?.revisions[0].effectiveDate).toBe('1970-01-01');
		expect(legacy?.revisions[0].workMode).toBe('flexible');
	});

	test('promotes a flat stored policy to the revision that has always applied', () => {
		const policies = parseSupabaseWorkPolicies([
			{
				member_id: 'member-current',
				work_hours: [[[], [], [], [], [], null, null]],
				minimum_daily_minutes: 480,
				work_mode: 'autonomous',
				work_policy: {
					workMode: 'autonomous',
					workingWeekdays: [1, 2, 3, 4, 5],
					dailyTargetMinutes: 0,
					weeklyTargetMinutes: 0,
					referenceStartTime: '09:00',
					fixedStartTime: '',
					fixedEndTime: '',
					coreTimeEnabled: false,
					coreStartTime: '',
					coreEndTime: '',
					breakPeriods: [],
					nightStartTime: '21:00',
					nightEndTime: '05:00'
				}
			}
		]);

		const current = policies.get('member-current');
		expect(current?.workMode).toBe('autonomous');
		expect(current?.currentPolicy.nightStartTime).toBe('21:00');
		expect(current?.currentPolicy.nightEndTime).toBe('05:00');
		expect(current?.revisions).toHaveLength(1);
		expect(current?.revisions[0].effectiveDate).toBe('1970-01-01');
	});

	test('reads a stored policy that already carries its revisions', () => {
		const revision = (effectiveDate: string, workMode: string) => ({
			effectiveDate,
			workMode,
			workingWeekdays: [1, 2, 3, 4, 5],
			dailyTargetMinutes: workMode === 'autonomous' ? 0 : 480,
			weeklyTargetMinutes: workMode === 'autonomous' ? 0 : 2400,
			referenceStartTime: '09:00',
			fixedStartTime: '',
			fixedEndTime: '',
			coreTimeEnabled: false,
			coreStartTime: '',
			coreEndTime: '',
			breakPeriods: [],
			nightStartTime: '22:00',
			nightEndTime: '06:00'
		});
		const policies = parseSupabaseWorkPolicies([
			{
				member_id: 'member-revised',
				work_hours: null,
				minimum_daily_minutes: 480,
				work_mode: 'autonomous',
				work_policy: {
					version: 1,
					revisions: [revision('2026-08-01', 'autonomous'), revision('1970-01-01', 'flexible')]
				}
			}
		]);

		const revised = policies.get('member-revised');
		expect(revised?.revisions.map((each) => each.effectiveDate)).toEqual([
			'1970-01-01',
			'2026-08-01'
		]);
		expect(revised?.currentPolicy.workMode).toBe('autonomous');
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
