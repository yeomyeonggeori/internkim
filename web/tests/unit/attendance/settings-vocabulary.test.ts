import { describe, expect, test } from 'bun:test';

import { attendanceWorkModes } from '../../../src/lib/attendance/work-mode';
import {
	WorkspaceAttendanceWorkMode,
	WorkspaceLeaveBalanceTracking,
	WorkspaceLeaveUnit
} from '../../../src/lib/server/public-api/catalog/settings';
import { defaultLeavePolicy } from '../../../src/lib/attendance/leave-policy-defaults';

describe('the ways a company can be said to work', () => {
	test('are the ones attendance_work_policy_set publishes, and no others', () => {
		expect([...attendanceWorkModes].sort()).toEqual(
			Object.values(WorkspaceAttendanceWorkMode).sort()
		);
	});
});

describe('the leave vocabulary the screens ship with', () => {
	test('tracks a balance in a way attendance_leave_policy_set publishes', () => {
		const tracking: string[] = Object.values(WorkspaceLeaveBalanceTracking);

		expect(tracking).toContain(defaultLeavePolicy().balanceTrackingMode);
	});

	test('takes leave in portions of a day attendance_leave_policy_set publishes', () => {
		const units: string[] = Object.values(WorkspaceLeaveUnit);
		const shipped = defaultLeavePolicy().leaveTypes.flatMap((leaveType) => leaveType.allowedUnits);

		expect(shipped.filter((unit) => !units.includes(unit))).toEqual([]);
	});
});
