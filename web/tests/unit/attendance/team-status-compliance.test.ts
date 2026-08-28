import { describe, expect, test } from 'bun:test';
import { teamComplianceMarks } from '../../../src/routes/attendance/team/team-status-compliance';
import type {
	AttendanceEmployeeWorkStatus,
	AttendanceWorkDayStatus
} from '../../../src/routes/attendance/attendance-api';

function dayOf(date: string, overrides: Partial<AttendanceWorkDayStatus>): AttendanceWorkDayStatus {
	return {
		date,
		workMode: 'fixed',
		hasBaseline: true,
		workingDate: true,
		targetMinutes: 480,
		actualMinutes: 480,
		actualSeconds: 480 * 60,
		provisionalMinutes: 0,
		provisionalSeconds: 0,
		leaveMinutes: 0,
		fulfilledMinutes: 480,
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
		hasIncompleteWorkRecord: false,
		status: 'fulfilled',
		workSegments: [],
		leaveSegments: [],
		...overrides
	};
}

function employeeOf(email: string, days: AttendanceWorkDayStatus[]): AttendanceEmployeeWorkStatus {
	return {
		email,
		displayName: email.split('@')[0],
		periodStart: days[0]?.date ?? '',
		periodEnd: days[days.length - 1]?.date ?? '',
		workMode: 'fixed',
		hasBaseline: true,
		targetMinutes: 0,
		actualMinutes: 0,
		actualSeconds: 0,
		provisionalMinutes: 0,
		provisionalSeconds: 0,
		workingCapacitySeconds: 0,
		calendarCapacitySeconds: 0,
		leaveMinutes: 0,
		fulfilledMinutes: 0,
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
		status: 'off',
		days
	};
}

const employees = [
	employeeOf('one@example.com', [
		dayOf('2026-08-17', { late: true }),
		dayOf('2026-08-18', { late: true, earlyLeave: true }),
		dayOf('2026-08-19', {})
	]),
	employeeOf('two@example.com', [
		dayOf('2026-08-17', {}),
		dayOf('2026-08-18', { workMode: 'flexible', coreTimeMissed: true })
	])
];

describe('teamComplianceMarks', () => {
	test('a day that carries one state is marked with it', () => {
		expect(teamComplianceMarks(employees, 'one@example.com', '2026-08-17')).toEqual(['late']);
	});

	test('a day that carries two states is marked with both in order', () => {
		expect(teamComplianceMarks(employees, 'one@example.com', '2026-08-18')).toEqual([
			'late',
			'earlyLeave'
		]);
	});

	test('a compliant day is not marked', () => {
		expect(teamComplianceMarks(employees, 'one@example.com', '2026-08-19')).toEqual([]);
	});

	test('marks belong to the person who earned them', () => {
		expect(teamComplianceMarks(employees, 'two@example.com', '2026-08-17')).toEqual([]);
		expect(teamComplianceMarks(employees, 'two@example.com', '2026-08-18')).toEqual([
			'coreTimeMissed'
		]);
	});

	test('a date outside the period is not marked', () => {
		expect(teamComplianceMarks(employees, 'one@example.com', '2026-09-01')).toEqual([]);
	});

	test('an unknown person is not marked', () => {
		expect(teamComplianceMarks(employees, 'nobody@example.com', '2026-08-18')).toEqual([]);
	});

	test('an empty payload marks nothing', () => {
		expect(teamComplianceMarks([], 'one@example.com', '2026-08-18')).toEqual([]);
	});

	test('asking twice for the same payload answers the same', () => {
		expect(teamComplianceMarks(employees, 'one@example.com', '2026-08-18')).toEqual(
			teamComplianceMarks(employees, 'one@example.com', '2026-08-18')
		);
	});
});
