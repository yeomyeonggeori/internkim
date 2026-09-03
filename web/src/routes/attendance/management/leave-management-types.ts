import type {
	EmployeeLeaveBalanceTrackingMode,
	EmployeeLeaveRequest,
	EmployeeLeaveType
} from '../leave/employee-leave-types';

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

export type LeaveManagementDetail = {
	employee: LeaveManagementEmployee;
	requests: EmployeeLeaveRequest[];
};

export type LeaveManagementPayload = {
	balanceTrackingMode: EmployeeLeaveBalanceTrackingMode;
	leaveTypes: EmployeeLeaveType[];
	employees: LeaveManagementEmployee[];
	detail?: LeaveManagementDetail;
};

export type LeaveManagementAdjustment = {
	employeeEmail: string;
	leaveTypeID: string;
	amountMilliDays: number;
	kind: 'adjustment';
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
};
