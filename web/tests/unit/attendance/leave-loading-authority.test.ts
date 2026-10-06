import { afterEach, beforeEach, expect, mock, spyOn, test } from 'bun:test';
import { ToolRefused } from '../../../src/lib/public-api-call';
import * as approvals from '../../../src/routes/attendance/approval/leave-approval-api';
import * as employee from '../../../src/routes/attendance/leave/employee-leave-api';
import * as management from '../../../src/routes/attendance/management/leave-management-api';
import { LeaveApprovalState } from '../../../src/routes/attendance/approval/leave-approval-state.svelte';
import { EmployeeLeaveState } from '../../../src/routes/attendance/leave/employee-leave-state.svelte';
import { LeaveManagementState } from '../../../src/routes/attendance/management/leave-management-state.svelte';
import { attendanceText } from '../../../src/routes/attendance/text';
import type { LeaveApprovalRequest } from '../../../src/routes/attendance/approval/leave-approval-types';

const approvedRequest: LeaveApprovalRequest = { id: 'sample-request', employeeEmail: 'sample@example.com', employeeName: '이샘플', leaveTypeID: 'annual', leaveTypeName: 'Annual', balanceMode: 'annual', status: 'approved', unit: 'fullDay', startDate: '2026-10-15', deductionMilliDays: 1000, reason: '', createdAt: '2026-10-06T03:00:00Z', updatedAt: '2026-10-06T03:00:00Z' };

let originalState: unknown;
beforeEach(() => { originalState = Reflect.get(globalThis, '$state'); Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value); });
afterEach(() => { mock.restore(); if (originalState === undefined) Reflect.deleteProperty(globalThis, '$state'); else Reflect.set(globalThis, '$state', originalState); });

for (const status of [401, 403]) {
	test(`leave read surfaces discard previously authorized data on ${status}`, async () => {
		const approvalState = new LeaveApprovalState(attendanceText.en.approval);
		const approvalRead = spyOn(approvals, 'fetchLeaveApprovalInbox').mockResolvedValue({ pending: [], pendingCount: 0 });
		await approvalState.load();
		approvalRead.mockRejectedValue(new Error('temporary outage'));
		await approvalState.load();
		expect(approvalState.inbox).not.toBeNull();
		approvalRead.mockRejectedValue(new ToolRefused('forbidden', 'FORBIDDEN', status));
		await approvalState.load();
		expect(approvalState.inbox).toBeNull();

		const employeeState = new EmployeeLeaveState(attendanceText.en.leave);
		const employeeRead = spyOn(employee, 'fetchEmployeeLeave').mockResolvedValue({ balanceTrackingMode: 'managed', leaveTypes: [], requests: [], summary: { usedMilliDays: 1000, reservedMilliDays: 0, availableMilliDays: 14000 } });
		await employeeState.load();
		employeeRead.mockRejectedValue(new Error('temporary outage'));
		await employeeState.load();
		expect(employeeState.payload).not.toBeNull();
		employeeRead.mockRejectedValue(new ToolRefused('forbidden', 'FORBIDDEN', status));
		await employeeState.load();
		expect(employeeState.payload).toBeNull();

		const managementState = new LeaveManagementState(attendanceText.en.management);
		const managementRead = spyOn(management, 'fetchLeaveManagement').mockResolvedValue({ balanceTrackingMode: 'managed', leaveTypes: [], employees: [] });
		await managementState.load();
		managementRead.mockRejectedValue(new Error('temporary outage'));
		await managementState.load();
		expect(managementState.payload).not.toBeNull();
		managementRead.mockRejectedValue(new ToolRefused('forbidden', 'FORBIDDEN', status));
		await managementState.load();
		expect(managementState.payload).toBeNull();
		expect(managementState.selectedEmployeeEmail).toBe('');
	});
	test(`post-mutation ${status} replacement failures settle in an error, not another loading state`, async () => {
		const approvalState = new LeaveApprovalState(attendanceText.en.approval);
		approvalState.inbox = { pending: [], pendingCount: 0 };
		// A write may succeed just before the next read loses authority.
		spyOn(approvals, 'decideLeaveApproval').mockResolvedValue(approvedRequest);
		spyOn(approvals, 'fetchLeaveApprovalInbox').mockRejectedValue(new ToolRefused('forbidden', 'FORBIDDEN', status));
		await expect(approvalState.decide('sample-request', { action: 'approve' })).rejects.toThrow();
		expect(approvalState.inbox).toBeNull();
		expect(approvalState.isLoading).toBe(false);
		expect(approvalState.errorMessage).not.toBe('');
		const employeeState = new EmployeeLeaveState(attendanceText.en.leave);
		employeeState.payload = { balanceTrackingMode: 'managed', leaveTypes: [], requests: [], summary: { usedMilliDays: 0, reservedMilliDays: 0, availableMilliDays: 15000 } };
		spyOn(employee, 'cancelEmployeeLeaveRequest').mockResolvedValue(undefined);
		spyOn(employee, 'fetchEmployeeLeave').mockRejectedValue(new ToolRefused('forbidden', 'FORBIDDEN', status));
		await expect(employeeState.cancel('sample-request')).rejects.toThrow();
		expect(employeeState.payload).toBeNull();
		expect(employeeState.isLoading).toBe(false);
		expect(employeeState.errorMessage).not.toBe('');
	});
}
