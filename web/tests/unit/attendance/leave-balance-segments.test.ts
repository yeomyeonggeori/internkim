import { describe, expect, test } from 'bun:test';
import { leaveBalanceSegments } from '../../../src/routes/attendance/leave/leave-balance-segments';

function rounded(value: number): number {
	return Math.round(value * 1_000_000) / 1_000_000;
}

function widthTotal(segments: ReturnType<typeof leaveBalanceSegments>): number {
	return rounded(segments.usedPercent + segments.reservedPercent + segments.availablePercent);
}

describe('leaveBalanceSegments', () => {
	test('draws pending inside remaining rather than beside it', () => {
		const segments = leaveBalanceSegments(
			{ usedMilliDays: 0, reservedMilliDays: 5000, availableMilliDays: 15000 }
		);

		expect(segments.usedPercent).toBe(0);
		expect(rounded(segments.reservedPercent)).toBe(rounded(100 / 3));
		expect(rounded(segments.availablePercent)).toBe(rounded(200 / 3));
		expect(widthTotal(segments)).toBe(100);
	});

	test('fills exactly the grant when leave has been used', () => {
		const segments = leaveBalanceSegments(
			{ usedMilliDays: 1000, reservedMilliDays: 500, availableMilliDays: 14000 }
		);

		expect(rounded(segments.usedPercent)).toBe(rounded((1000 / 15000) * 100));
		expect(rounded(segments.reservedPercent)).toBe(rounded((500 / 15000) * 100));
		expect(widthTotal(segments)).toBe(100);
	});

	test('never draws more pending than there is remaining', () => {
		const segments = leaveBalanceSegments(
			{ usedMilliDays: 1000, reservedMilliDays: 90000, availableMilliDays: 2000 }
		);

		expect(rounded(segments.reservedPercent)).toBe(rounded((2000 / 3000) * 100));
		expect(segments.availablePercent).toBe(0);
		expect(widthTotal(segments)).toBe(100);
	});

	test('draws no negative width when more leave was approved than granted', () => {
		const segments = leaveBalanceSegments(
			{ usedMilliDays: 20000, reservedMilliDays: 0, availableMilliDays: -5000 }
		);

		expect(segments.usedPercent).toBe(100);
		expect(segments.reservedPercent).toBe(0);
		expect(segments.availablePercent).toBe(0);
		for (const width of Object.values(segments)) expect(width >= 0).toBe(true);
	});
});
