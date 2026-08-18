import { beforeEach, describe, expect, mock, test } from 'bun:test';
import type {
	LeaveManagementAdjustment,
	LeaveManagementPastLeave,
	LeaveManagementPayload,
	LeaveManagementTimeCorrection
} from '../../../src/routes/attendance/management/leave-management-types';

const calls: Array<{ name: string; input: unknown }> = [];

mock.module('$lib/supabase', () => ({ isSupabaseConfigured: () => true }));
mock.module('../../../src/routes/attendance/leave/employee-leave-api', () => ({
	EmployeeLeaveAPIError: class EmployeeLeaveAPIError extends Error {}
}));
mock.module('$lib/attendance/supabase-leave-management', () => ({
	supabaseLeaveManagement: async (email: string) => {
		calls.push({ name: 'fetch', input: email });
		return { balanceTrackingMode: 'managed', leaveTypes: [], employees: [] } as LeaveManagementPayload;
	},
	adjustSupabaseManagedLeave: async (input: LeaveManagementAdjustment) => {
		calls.push({ name: 'adjust', input });
	},
	createSupabaseManagedPastLeave: async (input: LeaveManagementPastLeave) => {
		calls.push({ name: 'past', input });
	},
	cancelSupabaseManagedLeave: async (requestID: string, email: string) => {
		calls.push({ name: 'cancel', input: { requestID, email } });
	},
	correctSupabaseManagedLeaveTime: async (requestID: string, input: LeaveManagementTimeCorrection) => {
		calls.push({ name: 'correct', input: { requestID, input } });
	}
}));

const api = await import('../../../src/routes/attendance/management/leave-management-api');

beforeEach(() => {
	calls.length = 0;
});

describe('leave management API central plane routing', () => {
	test('routes reads and every management mutation to the Supabase workflow', async () => {
		const adjustment: LeaveManagementAdjustment = {
			employeeEmail: 'staff@example.com',
			leaveTypeID: 'leave',
			amountMilliDays: 1000,
			kind: 'adjustment',
			reason: 'grant',
			effectiveOn: '2026-08-18',
			expiresOn: ''
		};
		const pastLeave: LeaveManagementPastLeave = {
			employeeEmail: 'staff@example.com',
			leaveTypeID: 'leave',
			unit: 'halfDay',
			startDate: '2026-08-17',
			endDate: '2026-08-17',
			partialPeriod: 'custom',
			startTime: '10:00',
			reason: 'record'
		};
		const correction: LeaveManagementTimeCorrection = {
			employeeEmail: 'staff@example.com',
			startTime: '11:00',
			endTime: '15:00',
			reason: 'correct'
		};

		await api.fetchLeaveManagement('staff@example.com');
		await api.adjustManagedLeave(adjustment);
		await api.createManagedPastLeave(pastLeave);
		await api.cancelManagedLeaveRequest('leave-1', 'staff@example.com');
		await api.correctManagedLeaveTime('leave-1', correction);

		expect(calls).toEqual([
			{ name: 'fetch', input: 'staff@example.com' },
			{ name: 'adjust', input: adjustment },
			{ name: 'past', input: pastLeave },
			{ name: 'cancel', input: { requestID: 'leave-1', email: 'staff@example.com' } },
			{ name: 'correct', input: { requestID: 'leave-1', input: correction } }
		]);
	});
});
