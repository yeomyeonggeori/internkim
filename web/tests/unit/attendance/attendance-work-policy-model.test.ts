import { describe, expect, test } from 'bun:test';
import {
	copyAttendanceWorkPolicyRevision,
	currentAttendanceWorkPolicyRevision,
	attendanceMonthlyTargetMinutes,
	fixedAttendanceTargetMinutes,
	setAttendanceWorkMode,
	setAttendanceWorkingWeekdays,
	setAttendanceWeeklyTargetMinutes
} from '../../../src/routes/admin/attendance-work-policy-model';
import type {
	AttendanceWorkPolicy,
	AttendanceWorkPolicyRevision
} from '../../../src/routes/admin/admin-types';

function revision(): AttendanceWorkPolicyRevision {
	return {
		effectiveDate: '1970-01-01',
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
		nightEndTime: '06:00'
	};
}

describe('attendance work policy model', () => {
	test('selects the latest revision without sharing mutable arrays', () => {
		const first = revision();
		const second = { ...revision(), effectiveDate: '2026-07-31', workingWeekdays: [1, 2, 3] };
		const policy: AttendanceWorkPolicy = {
			version: 1,
			updatedAt: '',
			revisions: [first, second]
		};

		const selected = currentAttendanceWorkPolicyRevision(policy);
		selected.workingWeekdays.push(4);

		expect(selected.effectiveDate).toBe('2026-07-31');
		expect(second.workingWeekdays).toEqual([1, 2, 3]);
		expect(copyAttendanceWorkPolicyRevision(first)).not.toBe(first);
	});

	test('keeps flexible daily and weekly targets consistent with weekdays', () => {
		const changedWeekdays = setAttendanceWorkingWeekdays(revision(), [1, 2, 3, 4]);
		expect(changedWeekdays.dailyTargetMinutes).toBe(600);
		expect(changedWeekdays.weeklyTargetMinutes).toBe(2400);

		const changedTarget = setAttendanceWeeklyTargetMinutes(changedWeekdays, 1920);
		expect(changedTarget.dailyTargetMinutes).toBe(480);
		expect(changedTarget.weeklyTargetMinutes).toBe(1920);

		const roundedTarget = setAttendanceWeeklyTargetMinutes(
			{ ...revision(), workingWeekdays: [1, 2, 3, 4, 5, 6] },
			2415
		);
		expect(roundedTarget.dailyTargetMinutes).toBe(403);
		expect(roundedTarget.weeklyTargetMinutes).toBe(2418);
		expect(
			roundedTarget.dailyTargetMinutes * roundedTarget.workingWeekdays.length
		).toBe(roundedTarget.weeklyTargetMinutes);
	});

	test('derives fixed target after subtracting overlapping breaks', () => {
		expect(fixedAttendanceTargetMinutes('09:00', '18:00', [
			{ startTime: '12:00', endTime: '13:00' }
		])).toBe(480);
	});

	test('derives a monthly target from actual working dates and company holidays', () => {
		const policyRevision = revision();

		expect(attendanceMonthlyTargetMinutes(policyRevision, '2026-07', ['2026-07-17'])).toBe(22 * 480);
	});

	test('normalizes fields when changing work mode', () => {
		const autonomous = setAttendanceWorkMode(revision(), 'autonomous');
		expect(autonomous.weeklyTargetMinutes).toBe(0);
		expect(autonomous.coreTimeEnabled).toBe(false);

		const fixed = setAttendanceWorkMode(revision(), 'fixed');
		expect(fixed.fixedStartTime).toBe('09:00');
		expect(fixed.fixedEndTime).toBe('18:00');
		expect(fixed.dailyTargetMinutes).toBe(480);
		expect(fixed.weeklyTargetMinutes).toBe(2400);
	});
});
