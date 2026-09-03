import { companySettings } from '$lib/company/company-settings';
import { annualLeaveTypeID } from './leave-policy-defaults';
import { leavePolicyOf } from './supabase-leave-policy-settings';
import type { AttendanceLeavePolicy, LeaveType } from '../../routes/admin/admin-types';
import type {
	EmployeeLeaveBalanceTrackingMode,
	EmployeeLeaveSummary,
	EmployeeLeaveType
} from '../../routes/attendance/leave/employee-leave-types';

export type LeaveTypeDirectory = {
	trackingMode: EmployeeLeaveBalanceTrackingMode;
	offered: EmployeeLeaveType[];
	nameOf: (leaveTypeID: string) => string;
	ownsAnnualBalance: (leaveTypeID: string) => boolean;
	deductsAnnualBalance: (leaveTypeID: string) => boolean;
	isPaid: (leaveTypeID: string) => boolean;
};

export async function supabaseLeaveTypeDirectory(): Promise<LeaveTypeDirectory> {
	const settings = await companySettings();
	return leaveTypeDirectory(
		leavePolicyOf(settings),
		settings.leaveDays === null ? 'unlimited' : 'managed'
	);
}

export function leaveTypeDirectory(
	policy: AttendanceLeavePolicy,
	trackingMode: EmployeeLeaveBalanceTrackingMode
): LeaveTypeDirectory {
	const configured = new Map(policy.leaveTypes.map((leaveType) => [leaveType.id, leaveType]));
	const ownsAnnualBalance = (leaveTypeID: string) =>
		trackingMode === 'managed' && leaveTypeID === annualLeaveTypeID;
	return {
		trackingMode,
		offered: policy.leaveTypes
			.filter((leaveType) => leaveType.isActive)
			.map((leaveType) => employeeLeaveTypeOf(leaveType, ownsAnnualBalance(leaveType.id))),
		nameOf: (leaveTypeID) => configured.get(leaveTypeID)?.name ?? leaveTypeID,
		ownsAnnualBalance,
		deductsAnnualBalance: (leaveTypeID) =>
			configured.get(leaveTypeID)?.balanceMode === 'annual',
		isPaid: (leaveTypeID) => configured.get(leaveTypeID)?.paid ?? true
	};
}

export function withAnnualBalance(
	leaveTypes: EmployeeLeaveType[],
	directory: LeaveTypeDirectory,
	balance: EmployeeLeaveSummary
): EmployeeLeaveType[] {
	return leaveTypes.map((leaveType) =>
		directory.ownsAnnualBalance(leaveType.id) ? { ...leaveType, balance } : leaveType
	);
}

function employeeLeaveTypeOf(leaveType: LeaveType, ownsAnnualBalance: boolean): EmployeeLeaveType {
	return {
		id: leaveType.id,
		name: leaveType.name,
		balanceMode: ownsAnnualBalance ? 'annual' : 'none',
		allowedUnits: [...leaveType.allowedUnits],
		includeInSummary: ownsAnnualBalance,
		isActive: true
	};
}
