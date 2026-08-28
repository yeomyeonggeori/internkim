import { describe, expect, test } from 'bun:test';
import {
	companyWeekday,
	initialEffectiveDate,
	revisionForDate,
	workingDateForDate,
	workModeForDate,
	type AttendanceWorkPolicyRevision
} from '../../../src/lib/attendance/work-calendar-derivation';

function revision(
	effectiveDate: string,
	overrides: Partial<AttendanceWorkPolicyRevision> = {}
): AttendanceWorkPolicyRevision {
	return {
		effectiveDate,
		workMode: 'flexible',
		workingWeekdays: [1, 2, 3, 4, 5],
		dailyTargetMinutes: 480,
		weeklyTargetMinutes: 2400,
		referenceStartTime: '09:00',
		fixedStartTime: '',
		fixedEndTime: '',
		coreTimeEnabled: true,
		coreStartTime: '11:00',
		coreEndTime: '16:00',
		breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
		nightStartTime: '22:00',
		nightEndTime: '06:00',
		...overrides
	};
}

const noHolidays = new Set<string>();

describe('companyWeekday', () => {
	test('counts Monday as 1 and Sunday as 7', () => {
		expect(companyWeekday('2026-08-03')).toBe(1);
		expect(companyWeekday('2026-08-08')).toBe(6);
		expect(companyWeekday('2026-08-09')).toBe(7);
	});
});

describe('revisionForDate', () => {
	const revisions = [
		revision(initialEffectiveDate, { workMode: 'fixed' }),
		revision('2026-08-01', { workMode: 'autonomous' }),
		revision('2026-09-01', { workMode: 'flexible' })
	];

	test('the day a revision takes effect already uses it', () => {
		expect(revisionForDate(revisions, '2026-08-01').workMode).toBe('autonomous');
		expect(revisionForDate(revisions, '2026-09-01').workMode).toBe('flexible');
	});

	test('the day before a revision keeps the one it replaced', () => {
		expect(revisionForDate(revisions, '2026-07-31').workMode).toBe('fixed');
		expect(revisionForDate(revisions, '2026-08-31').workMode).toBe('autonomous');
	});

	test('the earliest revision covers everything before it', () => {
		expect(revisionForDate(revisions, '1999-01-01').workMode).toBe('fixed');
	});

	test('a date after the last revision keeps the last one', () => {
		expect(revisionForDate(revisions, '2030-01-01').workMode).toBe('flexible');
	});

	test('a policy with no revisions is not a policy', () => {
		expect(() => revisionForDate([], '2026-08-01')).toThrow('at least one revision');
	});
});

describe('workModeForDate', () => {
	test('reads the mode off the revision in force', () => {
		const revisions = [
			revision(initialEffectiveDate, { workMode: 'fixed' }),
			revision('2026-08-01', { workMode: 'autonomous' })
		];
		expect(workModeForDate(revisions, '2026-07-31')).toBe('fixed');
		expect(workModeForDate(revisions, '2026-08-01')).toBe('autonomous');
	});
});

describe('workingDateForDate', () => {
	test('a weekday the policy works is a working date', () => {
		expect(workingDateForDate(revision(initialEffectiveDate), '2026-08-03', noHolidays)).toBe(true);
	});

	test('a weekday the policy does not work is not', () => {
		expect(workingDateForDate(revision(initialEffectiveDate), '2026-08-08', noHolidays)).toBe(false);
	});

	test('a holiday is never a working date, whatever the weekday', () => {
		const holidays = new Set(['2026-08-17']);
		expect(workingDateForDate(revision(initialEffectiveDate), '2026-08-17', holidays)).toBe(false);
	});

	test('a revision that works weekends works them', () => {
		const weekends = revision(initialEffectiveDate, {
			workingWeekdays: [6, 7],
			weeklyTargetMinutes: 960
		});
		expect(workingDateForDate(weekends, '2026-08-08', noHolidays)).toBe(true);
		expect(workingDateForDate(weekends, '2026-08-03', noHolidays)).toBe(false);
	});

	test('a working-weekday change does not reach back past its effective date', () => {
		const revisions = [
			revision(initialEffectiveDate),
			revision('2026-08-08', { workingWeekdays: [1, 2, 3, 4, 5, 6], weeklyTargetMinutes: 2880 })
		];
		const earlierSaturday = '2026-08-01';
		const laterSaturday = '2026-08-08';
		expect(companyWeekday(earlierSaturday)).toBe(6);
		expect(companyWeekday(laterSaturday)).toBe(6);

		expect(
			workingDateForDate(revisionForDate(revisions, earlierSaturday), earlierSaturday, noHolidays)
		).toBe(false);
		expect(
			workingDateForDate(revisionForDate(revisions, laterSaturday), laterSaturday, noHolidays)
		).toBe(true);
	});
});
