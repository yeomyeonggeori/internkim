import { describe, expect, test } from 'bun:test';
import { workStatusCompliance } from '../../../src/routes/attendance/work-status/work-status-compliance';
import type { AttendanceWorkDayStatus } from '../../../src/routes/attendance/attendance-api';

function dayOf(overrides: Partial<AttendanceWorkDayStatus>): AttendanceWorkDayStatus {
	return {
		date: '2026-08-20',
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

describe('workStatusCompliance', () => {
	test('a compliant period reports nothing', () => {
		expect(workStatusCompliance([dayOf({}), dayOf({ date: '2026-08-21' })])).toEqual([]);
	});

	test('an empty period reports nothing', () => {
		expect(workStatusCompliance([])).toEqual([]);
	});

	test('each state is counted over the days that carry it', () => {
		expect(
			workStatusCompliance([
				dayOf({ date: '2026-08-17', late: true }),
				dayOf({ date: '2026-08-18', late: true, earlyLeave: true }),
				dayOf({ date: '2026-08-19' })
			])
		).toEqual([
			{ kind: 'late', dayCount: 2 },
			{ kind: 'earlyLeave', dayCount: 1 }
		]);
	});

	test('the order is late, then early departure, then core time', () => {
		expect(
			workStatusCompliance([
				dayOf({ workMode: 'flexible', coreTimeMissed: true }),
				dayOf({ date: '2026-08-21', earlyLeave: true }),
				dayOf({ date: '2026-08-22', late: true })
			]).map((entry) => entry.kind)
		).toEqual(['late', 'earlyLeave', 'coreTimeMissed']);
	});

	test('an autonomous period reports nothing, because no day carries a state', () => {
		expect(
			workStatusCompliance([
				dayOf({ workMode: 'autonomous', hasBaseline: false, workingDate: true }),
				dayOf({ date: '2026-08-21', workMode: 'autonomous', hasBaseline: false })
			])
		).toEqual([]);
	});

	test('a fixed period never reports missed core time', () => {
		expect(
			workStatusCompliance([dayOf({ late: true })]).map((entry) => entry.kind)
		).not.toContain('coreTimeMissed');
	});

	test('a period mixing work modes counts each day under its own state', () => {
		expect(
			workStatusCompliance([
				dayOf({ date: '2026-08-17', workMode: 'fixed', late: true }),
				dayOf({ date: '2026-08-19', workMode: 'flexible', coreTimeMissed: true })
			])
		).toEqual([
			{ kind: 'late', dayCount: 1 },
			{ kind: 'coreTimeMissed', dayCount: 1 }
		]);
	});
});
