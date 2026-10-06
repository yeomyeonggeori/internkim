import { describe, expect, test } from 'bun:test';
import { leaveTypeDirectory, withAnnualBalance } from '../../../src/lib/attendance/supabase-leave-types';
import { defaultLeavePolicy } from '../../../src/lib/attendance/leave-policy-defaults';

describe('leaveTypeDirectory', () => {
	test('an annual request deducts even while the company tracks no balance', () => {
		const directory = leaveTypeDirectory(defaultLeavePolicy(), 'unlimited');
		expect(directory.deductsAnnualBalance('annual')).toBe(true);
		expect(directory.deductsAnnualBalance('sick')).toBe(false);
	});

	test('a managed company deducts the same requests', () => {
		const directory = leaveTypeDirectory(defaultLeavePolicy(), 'managed');
		expect(directory.deductsAnnualBalance('annual')).toBe(true);
		expect(directory.deductsAnnualBalance('sick')).toBe(false);
	});

	test('only a managed company shows the annual balance on its type', () => {
		expect(leaveTypeDirectory(defaultLeavePolicy(), 'unlimited').ownsAnnualBalance('annual')).toBe(false);
		expect(leaveTypeDirectory(defaultLeavePolicy(), 'managed').ownsAnnualBalance('annual')).toBe(true);
	});

	test('a kind no configured type names keeps the name the row carries', () => {
		const directory = leaveTypeDirectory(defaultLeavePolicy(), 'managed');
		expect(directory.nameOf('annual')).toBe('연차');
		expect(directory.nameOf('연차')).toBe('연차');
		expect(directory.nameOf('leave')).toBe('leave');
	});

	test('the annual type carries its used days even while the company tracks no balance', () => {
		const directory = leaveTypeDirectory(defaultLeavePolicy(), 'unlimited');
		const summary = { usedMilliDays: 2000, reservedMilliDays: 0, availableMilliDays: 0 };
		const offered = withAnnualBalance(directory.offered, summary);
		expect(offered.find((leaveType) => leaveType.id === 'annual')?.balance).toEqual(summary);
		expect(offered.find((leaveType) => leaveType.id === 'sick')?.balance).toBeUndefined();
	});
});
