import { describe, expect, test } from 'bun:test';
import { leaveBalanceSegments } from '../../../src/routes/attendance/leave/leave-balance-segments';

function widthTotal(segments: ReturnType<typeof leaveBalanceSegments>): number {
	return rounded(segments.usedPercent + segments.reservedPercent + segments.availablePercent);
}

function rounded(value: number): number {
	return Math.round(value * 1_000_000) / 1_000_000;
}

describe('leaveBalanceSegments', () => {
	test('draws pending inside remaining rather than beside it', () => {
		const segments = leaveBalanceSegments({
			usedMilliDays: 0,
			reservedMilliDays: 5000,
			availableMilliDays: 15000
		});

		expect(segments.usedPercent).toBe(0);
		expect(rounded(segments.reservedPercent)).toBe(rounded(100 / 3));
		expect(rounded(segments.availablePercent)).toBe(rounded(200 / 3));
		expect(rounded(widthTotal(segments))).toBe(rounded(100));
	});

	test('fills exactly the grant when leave has been used', () => {
		const segments = leaveBalanceSegments({
			usedMilliDays: 3000,
			reservedMilliDays: 2000,
			availableMilliDays: 12000
		});

		expect(rounded(segments.usedPercent)).toBe(rounded(20));
		expect(rounded(segments.reservedPercent)).toBe(rounded((2000 / 15000) * 100));
		expect(rounded(widthTotal(segments))).toBe(rounded(100));
	});

	test('never draws more pending than there is remaining', () => {
		const segments = leaveBalanceSegments({
			usedMilliDays: 1000,
			reservedMilliDays: 90000,
			availableMilliDays: 2000
		});

		expect(rounded(segments.reservedPercent)).toBe(rounded((2000 / 3000) * 100));
		expect(segments.availablePercent).toBe(0);
		expect(rounded(widthTotal(segments))).toBe(rounded(100));
	});

	test('draws nothing without a grant to draw against', () => {
		expect(leaveBalanceSegments(undefined)).toEqual({
			usedPercent: 0,
			reservedPercent: 0,
			availablePercent: 0
		});
		expect(
			leaveBalanceSegments({ usedMilliDays: 0, reservedMilliDays: 0, availableMilliDays: 0 })
		).toEqual({ usedPercent: 0, reservedPercent: 0, availablePercent: 0 });
	});
});
