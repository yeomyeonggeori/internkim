import { describe, expect, test } from 'bun:test';
import {
	leaveManagementEmployee,
	leaveManagementRequest,
	type SupabaseLeaveManagementRow
} from '../../../src/lib/attendance/supabase-leave-management-model';

const member = { id: 'member-1', name: '이샘플', email: 'sample@example.com' };

function leave(overrides: Partial<SupabaseLeaveManagementRow>): SupabaseLeaveManagementRow {
	return {
		id: 'leave-1',
		member_id: member.id,
		kind: 'leave',
		is_deducted: true,
		days: 1,
		status: 'approved',
		starts_at: '2026-08-10T00:00:00.000Z',
		ends_at: '2026-08-11T00:00:00.000Z',
		note: null,
		cancelled_at: null,
		...overrides
	};
}

describe('supabase leave management model', () => {
	test('counts only active current-year deducted leave in employee balances', () => {
		const employee = leaveManagementEmployee(
			member,
			[
				leave({ id: 'approved', days: 2 }),
				leave({ id: 'requested', status: 'requested', days: 0.5 }),
				leave({ id: 'cancelled', days: 1, cancelled_at: '2026-08-11T00:00:00.000Z' }),
				leave({ id: 'rejected', status: 'rejected', days: 1 }),
				leave({ id: 'non-deducted', is_deducted: false, days: 1 }),
				leave({ id: 'other-year', starts_at: '2025-12-10T00:00:00.000Z', ends_at: '2025-12-11T00:00:00.000Z', days: 1 })
			],
			2026,
			8,
			'UTC'
		);

		expect(employee).toMatchObject({
			grantedMilliDays: 10000,
			availableMilliDays: 7500,
			usedMilliDays: 2000,
			reservedMilliDays: 500
		});
	});

	test('maps cancelled leave requests to cancelled status', () => {
		const request = leaveManagementRequest(
			leave({ status: 'requested', cancelled_at: '2026-08-11T00:00:00.000Z' }),
			'UTC'
		);

		expect(request).toMatchObject({ status: 'cancelled', canCancel: false });
	});
});
