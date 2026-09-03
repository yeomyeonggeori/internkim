import { describe, expect, test } from 'bun:test';
import {
	defaultLeaveTypes,
	systemLeaveTypeKind
} from '../../../src/lib/attendance/leave-policy-defaults';

// admind declared these too and a conformance test held the two copies
// together. The device keeps no leave policy any more, so this module is the
// only place they are written.
describe('the leave types a new company is seeded with', () => {
	test('are all active and annual is the only one with a balance', () => {
		const seeded = defaultLeaveTypes();
		expect(seeded.length).toBeGreaterThan(0);
		expect(seeded.every((leaveType) => leaveType.isActive)).toBe(true);
		expect(seeded.filter((leaveType) => leaveType.balanceMode === 'annual')).toHaveLength(1);
	});

	test('each keeps the system kind it was issued with', () => {
		for (const leaveType of defaultLeaveTypes()) {
			expect(systemLeaveTypeKind(leaveType.id)).toBeDefined();
		}
	});
});

describe('a leave type id a company already issued', () => {
	test('keeps its kind even when a new company is no longer seeded with it', () => {
		expect(defaultLeaveTypes().some((leaveType) => leaveType.id === 'bereavement')).toBe(false);
		expect(systemLeaveTypeKind('bereavement')).toBe('bereavement');
	});

	test('is undefined when nobody ever issued it', () => {
		expect(systemLeaveTypeKind('custom-whatever')).toBeUndefined();
	});
});
