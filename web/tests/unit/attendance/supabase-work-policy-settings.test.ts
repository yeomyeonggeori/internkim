import { describe, expect, test } from 'bun:test';
import { workPolicyResponse } from '../../../src/lib/attendance/supabase-work-policy-settings';
import type { CompanySettings } from '../../../src/lib/company/company-settings';

function flatPolicy(workMode: 'autonomous' | 'flexible' | 'fixed' = 'autonomous') {
	return {
		workMode,
		workingWeekdays: [1, 2, 3, 4, 5],
		dailyTargetMinutes: workMode === 'autonomous' ? 0 : 480,
		weeklyTargetMinutes: workMode === 'autonomous' ? 0 : 2400,
		referenceStartTime: '09:00',
		fixedStartTime: workMode === 'fixed' ? '09:00' : '',
		fixedEndTime: workMode === 'fixed' ? '18:00' : '',
		coreTimeEnabled: false,
		coreStartTime: '',
		coreEndTime: '',
		breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
		nightStartTime: '22:00',
		nightEndTime: '06:00'
	};
}

function settingsWith(attendanceWorkPolicy: unknown): CompanySettings {
	return {
		rules: { attendanceWorkPolicy } as CompanySettings['rules'],
		timeZone: 'Asia/Seoul',
		leaveDays: null
	};
}

describe('workPolicyResponse', () => {
	test('reads a policy stored as the revisions the saver writes', () => {
		const response = workPolicyResponse(
			settingsWith({
				version: 1,
				revisions: [{ ...flatPolicy('autonomous'), effectiveDate: '1970-01-01' }]
			})
		);

		expect(response.policy.revisions).toHaveLength(1);
		expect(response.policy.revisions[0].workMode).toBe('autonomous');
		expect(response.policy.revisions[0].effectiveDate).toBe('1970-01-01');
	});

	test('keeps every revision, in the order they took effect', () => {
		const response = workPolicyResponse(
			settingsWith({
				version: 1,
				revisions: [
					{ ...flatPolicy('fixed'), effectiveDate: '2026-08-01' },
					{ ...flatPolicy('flexible'), effectiveDate: '1970-01-01' }
				]
			})
		);

		expect(response.policy.revisions.map((revision) => revision.effectiveDate)).toEqual([
			'1970-01-01',
			'2026-08-01'
		]);
		expect(response.policy.revisions[1].workMode).toBe('fixed');
	});

	test('still reads a policy stored flat, as a device on an older release sends it', () => {
		const response = workPolicyResponse(settingsWith(flatPolicy('flexible')));

		expect(response.policy.revisions).toHaveLength(1);
		expect(response.policy.revisions[0].workMode).toBe('flexible');
		expect(response.policy.revisions[0].effectiveDate).toBe('1970-01-01');
	});

	test('gives a company that has never saved one the default', () => {
		const response = workPolicyResponse(settingsWith(undefined));

		expect(response.policy.revisions).toHaveLength(1);
		expect(response.policy.revisions[0].effectiveDate).toBe('1970-01-01');
	});

	test('refuses a stored policy that is neither shape', () => {
		expect(() => workPolicyResponse(settingsWith({ version: 1, revisions: [] }))).toThrow(
			'attendance work policy revisions are invalid for this company'
		);
		expect(() => workPolicyResponse(settingsWith({ version: 1, revisions: [{}] }))).toThrow(
			'attendance work policy revision is invalid for this company'
		);
	});
});
