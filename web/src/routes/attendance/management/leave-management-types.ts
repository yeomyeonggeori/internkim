import type { EmployeeLeaveRequest, EmployeeLeaveType } from '../leave/employee-leave-types';

export type LeaveManagementBalance = {
	leaveTypeID: string;
	leaveTypeName: string;
	grantedMilliDays: number;
	availableMilliDays: number;
	reservedMilliDays: number;
	usedMilliDays: number;
	expiredMilliDays: number;
	nextExpiryDate?: string;
	nextExpiryMilliDays: number;
};

export type LeaveManagementEmployee = {
	email: string;
	displayName: string;
	grantedMilliDays: number;
	availableMilliDays: number;
	reservedMilliDays: number;
	usedMilliDays: number;
	expiringMilliDays: number;
	balances: LeaveManagementBalance[];
};

export type LeaveManagementLedgerEntry = {
	id: string;
	operationType: string;
	leaveTypeID: string;
	leaveTypeName: string;
	deltaMilliDays: number;
	balanceAfterMilliDays: number;
	effectiveOn: string;
	occurredAt: string;
	reason?: string;
};

export type LeaveManagementDetail = {
	employee: LeaveManagementEmployee;
	requests: EmployeeLeaveRequest[];
	ledgerEntries: LeaveManagementLedgerEntry[];
};

export type LeaveManagementPayload = {
	leaveTypes: EmployeeLeaveType[];
	employees: LeaveManagementEmployee[];
	detail?: LeaveManagementDetail;
};

export type LeaveManagementAdjustment = {
	employeeEmail: string;
	leaveTypeID: string;
	amountMilliDays: number;
	kind: 'adjustment' | 'legalCorrection';
	reason: string;
	effectiveOn: string;
	expiresOn: string;
};

export type LeaveManagementPastLeave = {
	employeeEmail: string;
	leaveTypeID: string;
	unit: 'fullDay' | 'halfDay' | 'quarterDay';
	startDate: string;
	endDate: string;
	partialPeriod: 'morning' | 'afternoon' | 'custom' | '';
	startTime: string;
	reason: string;
};

export type LeaveManagementTimeCorrection = {
	employeeEmail: string;
	startTime: string;
	endTime: string;
	reason: string;
};
