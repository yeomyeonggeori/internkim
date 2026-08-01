import { describe, expect, test } from 'bun:test';
import type { AttendanceEmployeeWorkStatus } from '../../../src/routes/attendance/attendance-api';
import { filterEmployeeWorkStatuses } from '../../../src/routes/attendance/employee-status/employee-work-status-model';
import {
	calculatePeriodCapacityMinutes,
	formatWorkStatusDuration
} from '../../../src/routes/attendance/work-status/work-status-format';

const employee = (
	displayName: string,
	email: string,
	status: string,
	overrides: Partial<AttendanceEmployeeWorkStatus> = {}
): AttendanceEmployeeWorkStatus => ({
	displayName,
	email,
	status,
	periodStart: '2026-07-27',
	periodEnd: '2026-08-02',
	workMode: 'flexible',
	hasBaseline: true,
	targetMinutes: 2400,
	actualMinutes: 2400,
	provisionalMinutes: 0,
	leaveMinutes: 0,
	fulfilledMinutes: 2400,
	differenceMinutes: 0,
	remainingMinutes: 0,
	overtimeMinutes: 0,
	nightMinutes: 0,
	isWorking: false,
	needsReview: false,
	coreTimeMissed: false,
	late: false,
	earlyLeave: false,
	hasLeaveWorkOverlap: false,
	hasIncompleteRecords: false,
	days: [],
	...overrides
});

describe('formatWorkStatusDuration', () => {
	test('keeps zero values visible in work status summaries', () => {
		expect(formatWorkStatusDuration(0, { hourUnit: '시간', minuteUnit: '분' })).toBe(
			'00시간 00분'
		);
	});
});

describe('calculatePeriodCapacityMinutes', () => {
	test('uses every calendar hour in day, week, and month periods', () => {
		expect(calculatePeriodCapacityMinutes('2026-07-31', '2026-07-31')).toBe(24 * 60);
		expect(calculatePeriodCapacityMinutes('2026-07-27', '2026-08-02')).toBe(168 * 60);
		expect(calculatePeriodCapacityMinutes('2026-05-01', '2026-05-31')).toBe(744 * 60);
	});
});

describe('filterEmployeeWorkStatuses', () => {
	test('matches employees by normalized name or email', () => {
		const employees = [
			employee('김민지', 'minji@example.com', 'fulfilled'),
			employee('박지훈', 'jihoon@example.com', 'overtime', { overtimeMinutes: 60 })
		];

		expect(filterEmployeeWorkStatuses(employees, ' 민지 ', 'all')).toEqual([employees[0]]);
		expect(filterEmployeeWorkStatuses(employees, 'JIHoon', 'all')).toEqual([employees[1]]);
	});

	test('filters by work status', () => {
		const employees = [
			employee('김민지', 'minji@example.com', 'fulfilled'),
			employee('박지훈', 'jihoon@example.com', 'overtime', { overtimeMinutes: 60 })
		];

		expect(filterEmployeeWorkStatuses(employees, '', 'overtime')).toEqual([employees[1]]);
	});

	test('filters review and schedule violations independently from the primary status', () => {
		const employees = [
			employee('김민지', 'minji@example.com', 'needsReview', { needsReview: true }),
			employee('박지훈', 'jihoon@example.com', 'remaining', { late: true }),
			employee('이서연', 'seoyeon@example.com', 'remaining', { coreTimeMissed: true }),
			employee('최도윤', 'doyoon@example.com', 'remaining', { earlyLeave: true }),
			employee('정하늘', 'haneul@example.com', 'remaining', { late: true, earlyLeave: true })
		];

		expect(filterEmployeeWorkStatuses(employees, '', 'needsReview')).toEqual([employees[0]]);
		expect(filterEmployeeWorkStatuses(employees, '', 'lateOrEarly')).toEqual([
			employees[1],
			employees[3],
			employees[4]
		]);
		expect(filterEmployeeWorkStatuses(employees, '', 'coreTimeMissed')).toEqual([employees[2]]);
	});
});
