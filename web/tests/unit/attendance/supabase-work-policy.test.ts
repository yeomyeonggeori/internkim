import { describe, expect, test } from 'bun:test';
import { parseSupabaseWorkPolicies } from '../../../src/lib/attendance/supabase-work-policy';

describe('parseSupabaseWorkPolicies', () => {
	test('preserves the company work mode returned for each member', () => {
		const policies = parseSupabaseWorkPolicies([
			{
				member_id: 'member-fixed',
				work_hours: null,
				minimum_daily_minutes: 480,
				work_mode: 'fixed',
				work_calendar: [
					{ date: '2027-01-01', workMode: 'fixed', workingDate: false }
				]
			},
			{
				member_id: 'member-autonomous',
				work_hours: [[null, null, null, null, null, null, null]],
				minimum_daily_minutes: null,
				work_mode: 'autonomous',
				work_calendar: null
			}
		]);

		expect(policies.get('member-fixed')?.workMode).toBe('fixed');
		expect(policies.get('member-fixed')?.workCalendar).toEqual([
			{ date: '2027-01-01', workMode: 'fixed', workingDate: false }
		]);
		expect(policies.get('member-autonomous')?.workMode).toBe('autonomous');
		expect(policies.get('member-autonomous')?.workCalendar).toBe(null);
	});

	test('keeps a legacy policy without a date projection usable', () => {
		const policies = parseSupabaseWorkPolicies([
			{
				member_id: 'member-legacy',
				work_hours: [[[], [], [], [], [], null, null]],
				minimum_daily_minutes: 480,
				work_mode: 'flexible'
			}
		]);

		expect(policies.get('member-legacy')?.workCalendar).toBe(null);
	});

	test('uses the one current JSON policy instead of a historical calendar mode', () => {
		const policies = parseSupabaseWorkPolicies([
			{
				member_id: 'member-current',
				work_hours: [[[], [], [], [], [], null, null]],
				minimum_daily_minutes: 480,
				work_mode: 'autonomous',
				work_calendar: [{ date: '2027-01-01', workMode: 'fixed', workingDate: true }],
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

		expect(policies.get('member-current')?.workMode).toBe('autonomous');
		expect(policies.get('member-current')?.currentPolicy.nightStartTime).toBe('21:00');
		expect(policies.get('member-current')?.currentPolicy.nightEndTime).toBe('05:00');
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
