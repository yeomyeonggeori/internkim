import { describe, expect, test } from 'bun:test';
import {
	leaveDaysInYear,
	leaveYearClosesOn,
	leaveYearOf,
	leaveYearOpensOn
} from '../../../src/lib/attendance/leave-year-share';

describe('the leave year a date falls in', () => {
	test('is the calendar year when the year starts in January', () => {
		expect(leaveYearOf('2026-02-15')).toBe(2026);
		expect(leaveYearOf('2026-12-31')).toBe(2026);
	});

	test('is the year it opened in when the year starts later', () => {
		expect(leaveYearOf('2026-02-15', 3, 1)).toBe(2025);
		expect(leaveYearOf('2026-03-01', 3, 1)).toBe(2026);
		expect(leaveYearOf('2026-02-28', 3, 1)).toBe(2025);
	});

	test('opens on the day the company says and closes the day before the next one', () => {
		expect(leaveYearOpensOn(2025, 3, 1)).toBe('2025-03-01');
		expect(leaveYearClosesOn(2025, 3, 1)).toBe('2026-02-28');
		expect(leaveYearClosesOn(2027, 3, 1)).toBe('2028-02-29');
		expect(leaveYearClosesOn(2026)).toBe('2026-12-31');
	});
});

describe('the share of a leave that falls in a leave year', () => {
	test('a February leave is the previous leave year for a March start', () => {
		expect(leaveDaysInYear(2, '2026-02-10', '2026-02-11', 2025, 3, 1)).toBe(2);
		expect(leaveDaysInYear(2, '2026-02-10', '2026-02-11', 2026, 3, 1)).toBe(0);
	});

	test('a leave over the boundary splits day by day', () => {
		expect(leaveDaysInYear(4, '2026-02-27', '2026-03-02', 2025, 3, 1)).toBe(2);
		expect(leaveDaysInYear(4, '2026-02-27', '2026-03-02', 2026, 3, 1)).toBe(2);
	});

	test('a leave over new year stays whole inside a March-to-March year', () => {
		expect(leaveDaysInYear(3, '2025-12-30', '2026-01-01', 2025, 3, 1)).toBe(3);
		expect(leaveDaysInYear(3, '2025-12-30', '2026-01-01', 2026, 3, 1)).toBe(0);
	});

	test('a January start counts exactly as before', () => {
		expect(leaveDaysInYear(3, '2025-12-30', '2026-01-01', 2025)).toBe(2);
		expect(leaveDaysInYear(3, '2025-12-30', '2026-01-01', 2026)).toBe(1);
	});
});
