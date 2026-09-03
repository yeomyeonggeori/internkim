import { describe, expect, test } from 'bun:test';
import { storedWorkPolicyRevisions } from '../../../src/lib/attendance/stored-work-policy';
import {
	workPolicyResponse,
	type AnsweredWorkPolicy
} from '../../../src/lib/attendance/supabase-work-policy-settings';
import type { CompanyHoliday } from '../../../src/routes/admin/admin-types';

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

function answeredWith(policy: AnsweredWorkPolicy['policy']): AnsweredWorkPolicy {
	return { timeZone: 'Asia/Seoul', workMode: 'flexible', policy, people: [] };
}

// The record normalizes what is stored before it answers, through this same
// function: web/src/lib/server/public-api/record/company-tools.ts calls it for
// attendance_work_policy_get. These cases are that normalization.
describe('storedWorkPolicyRevisions', () => {
	test('reads a policy stored as the revisions the saver writes', () => {
		const revisions = storedWorkPolicyRevisions(
			{ version: 1, revisions: [{ ...flatPolicy('autonomous'), effectiveDate: '1970-01-01' }] },
			'this company'
		);

		expect(revisions).toHaveLength(1);
		expect(revisions[0].workMode).toBe('autonomous');
		expect(revisions[0].effectiveDate).toBe('1970-01-01');
	});

	test('keeps every revision, in the order they took effect', () => {
		const revisions = storedWorkPolicyRevisions(
			{
				version: 1,
				revisions: [
					{ ...flatPolicy('fixed'), effectiveDate: '2026-08-01' },
					{ ...flatPolicy('flexible'), effectiveDate: '1970-01-01' }
				]
			},
			'this company'
		);

		expect(revisions.map((revision) => revision.effectiveDate)).toEqual([
			'1970-01-01',
			'2026-08-01'
		]);
		expect(revisions[1].workMode).toBe('fixed');
	});

	test('still reads a policy stored flat, as a device on an older release sends it', () => {
		const revisions = storedWorkPolicyRevisions(flatPolicy('flexible'), 'this company');

		expect(revisions).toHaveLength(1);
		expect(revisions[0].workMode).toBe('flexible');
		expect(revisions[0].effectiveDate).toBe('1970-01-01');
	});

	test('refuses a stored policy that is neither shape', () => {
		expect(() => storedWorkPolicyRevisions({ version: 1, revisions: [] }, 'this company')).toThrow(
			'attendance work policy revisions are invalid for this company'
		);
		expect(() => storedWorkPolicyRevisions({ version: 1, revisions: [{}] }, 'this company')).toThrow(
			'attendance work policy revision is invalid for this company'
		);
	});
});

describe('workPolicyResponse', () => {
	test('gives a company that has never saved one the default, covering all of time', () => {
		const response = workPolicyResponse(answeredWith(null), []);

		expect(response.policy.revisions).toHaveLength(1);
		expect(response.policy.revisions[0].effectiveDate).toBe('1970-01-01');
		expect(response.timeZone).toBe('Asia/Seoul');
	});

	test('answers the revisions the record resolved, untouched', () => {
		const revisions = [
			{ ...flatPolicy('flexible'), effectiveDate: '1970-01-01' },
			{ ...flatPolicy('fixed'), effectiveDate: '2026-08-01' }
		];

		const response = workPolicyResponse(answeredWith({ version: 1, revisions }), []);

		expect(response.policy.revisions).toEqual(revisions);
	});

	test('marks the holidays of the month the screen is showing', () => {
		const currentMonth = workPolicyResponse(answeredWith(null), []).currentMonth;
		const holiday: CompanyHoliday = {
			id: 'company-holiday-1',
			title: '창립기념일',
			date: `${currentMonth}-03`,
			recursAnnually: false,
			createdAt: '',
			updatedAt: ''
		};

		const response = workPolicyResponse(answeredWith(null), [holiday]);

		expect(response.holidayDates).toEqual([holiday.date]);
	});
});
