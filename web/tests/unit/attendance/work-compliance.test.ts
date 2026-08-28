import { describe, expect, test } from 'bun:test';
import { workCompliance } from '../../../src/lib/attendance/work-compliance';
import type { AttendanceWorkPolicyRevision } from '../../../src/lib/attendance/work-calendar-derivation';

const minute = (hour: number, minutes = 0) => hour * 60 + minutes;
const endOfDay = minute(23, 59);

function revisionOf(
	overrides: Partial<AttendanceWorkPolicyRevision> = {}
): AttendanceWorkPolicyRevision {
	return {
		effectiveDate: '1970-01-01',
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
		nightEndTime: '06:00',
		...overrides
	};
}

const flexible = revisionOf({
	workMode: 'flexible',
	fixedStartTime: '',
	fixedEndTime: '',
	coreTimeEnabled: true,
	coreStartTime: '11:00',
	coreEndTime: '16:00'
});

const worked = (startHour: number, endHour: number) => [
	{ startMinute: minute(startHour), endMinute: minute(endHour) }
];

describe('workCompliance on a fixed schedule', () => {
	test('a day worked start to end is compliant', () => {
		expect(workCompliance(revisionOf(), true, worked(9, 18), [], endOfDay)).toEqual({
			coreTimeMissed: false,
			late: false,
			earlyLeave: false
		});
	});

	test('arriving after the start is late', () => {
		expect(workCompliance(revisionOf(), true, worked(10, 18), [], endOfDay)).toMatchObject({
			late: true,
			earlyLeave: false
		});
	});

	test('leaving before the end is early leave', () => {
		expect(workCompliance(revisionOf(), true, worked(9, 17), [], endOfDay)).toMatchObject({
			late: false,
			earlyLeave: true
		});
	});

	test('a short middle of the day is both', () => {
		expect(workCompliance(revisionOf(), true, worked(10, 17), [], endOfDay)).toMatchObject({
			late: true,
			earlyLeave: true
		});
	});

	test('leave covering the whole day answers for both ends', () => {
		expect(
			workCompliance(revisionOf(), true, [], [{ startMinute: minute(9), endMinute: minute(18) }], endOfDay)
		).toMatchObject({ late: false, earlyLeave: false });
	});

	test('a morning leave answers for the start but not the end', () => {
		expect(
			workCompliance(revisionOf(), true, [], [{ startMinute: minute(9), endMinute: minute(14) }], endOfDay)
		).toMatchObject({ late: false, earlyLeave: true });
	});

	test('nothing is judged before the start has passed', () => {
		expect(workCompliance(revisionOf(), true, [], [], minute(8))).toEqual({
			coreTimeMissed: false,
			late: false,
			earlyLeave: false
		});
	});

	test('the end is not judged until it has passed', () => {
		expect(workCompliance(revisionOf(), true, worked(9, 12), [], minute(17))).toMatchObject({
			late: false,
			earlyLeave: false
		});
	});

	test('core time is never missed on a fixed schedule', () => {
		expect(
			workCompliance(revisionOf({ coreTimeEnabled: true, coreStartTime: '11:00', coreEndTime: '16:00' }), true, [], [], endOfDay)
		).toMatchObject({ coreTimeMissed: false });
	});
});

describe('workCompliance on a flexible schedule', () => {
	test('an empty core window is missed', () => {
		expect(workCompliance(flexible, true, worked(6, 10), [], endOfDay)).toMatchObject({
			coreTimeMissed: true,
			late: false,
			earlyLeave: false
		});
	});

	test('work covering core time apart from the break is compliant', () => {
		expect(workCompliance(flexible, true, worked(11, 16), [], endOfDay)).toMatchObject({
			coreTimeMissed: false
		});
	});

	test('the break inside core time does not have to be covered', () => {
		expect(
			workCompliance(
				flexible,
				true,
				[
					{ startMinute: minute(11), endMinute: minute(12) },
					{ startMinute: minute(13), endMinute: minute(16) }
				],
				[],
				endOfDay
			)
		).toMatchObject({ coreTimeMissed: false });
	});

	test('a gap inside core time outside the break is missed', () => {
		expect(
			workCompliance(
				flexible,
				true,
				[
					{ startMinute: minute(11), endMinute: minute(14) },
					{ startMinute: minute(15), endMinute: minute(16) }
				],
				[],
				endOfDay
			)
		).toMatchObject({ coreTimeMissed: true });
	});

	test('leave answers for core time', () => {
		expect(
			workCompliance(flexible, true, [], [{ startMinute: minute(9), endMinute: minute(18) }], endOfDay)
		).toMatchObject({ coreTimeMissed: false });
	});

	test('core time is not judged until it has passed', () => {
		expect(workCompliance(flexible, true, [], [], minute(15))).toMatchObject({
			coreTimeMissed: false
		});
	});

	test('a schedule without core time judges nothing', () => {
		expect(
			workCompliance(revisionOf({ workMode: 'flexible', coreTimeEnabled: false }), true, [], [], endOfDay)
		).toEqual({ coreTimeMissed: false, late: false, earlyLeave: false });
	});
});

describe('workCompliance where no rule applies', () => {
	test('an autonomous schedule judges nothing', () => {
		expect(workCompliance(revisionOf({ workMode: 'autonomous' }), true, [], [], endOfDay)).toEqual({
			coreTimeMissed: false,
			late: false,
			earlyLeave: false
		});
	});

	test('a non-working date judges nothing', () => {
		expect(workCompliance(revisionOf(), false, [], [], endOfDay)).toEqual({
			coreTimeMissed: false,
			late: false,
			earlyLeave: false
		});
	});
});
