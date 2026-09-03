export type EmployeeLeaveBalanceMode = 'annual' | 'separate' | 'none';
export type EmployeeLeaveBalanceTrackingMode = 'managed' | 'unlimited';
export type EmployeeLeaveUnit = 'fullDay' | 'halfDay' | 'quarterDay';
export type EmployeeLeaveStatus = 'pending' | 'approved' | 'rejected' | 'cancelled';
export type EmployeeLeavePartialPeriod = 'morning' | 'afternoon' | 'custom';
export const employeeLeaveErrorCodes = [
	'invalidInput',
	'leaveConflict',
	'workConflict',
	'insufficientBalance',
	'requestNotFound',
	'invalidStatus',
	'internal'
] as const;
export type EmployeeLeaveErrorCode = (typeof employeeLeaveErrorCodes)[number];

const employeeLeaveErrorCodeSet: ReadonlySet<string> = new Set(employeeLeaveErrorCodes);

export function isEmployeeLeaveErrorCode(value: unknown): value is EmployeeLeaveErrorCode {
	return typeof value === 'string' && employeeLeaveErrorCodeSet.has(value);
}

export type EmployeeLeaveType = {
	id: string;
	name: string;
	balanceMode: EmployeeLeaveBalanceMode;
	allowedUnits: EmployeeLeaveUnit[];
	includeInSummary: boolean;
	balance?: EmployeeLeaveSummary;
	isActive: boolean;
};

export type EmployeeLeaveSummary = {
	usedMilliDays: number;
	reservedMilliDays: number;
	availableMilliDays: number;
};

export type EmployeeLeaveRequest = {
	id: string;
	leaveTypeID: string;
	leaveTypeName: string;
	status: EmployeeLeaveStatus;
	unit: EmployeeLeaveUnit;
	startDate: string;
	endDate?: string;
	partialPeriod?: EmployeeLeavePartialPeriod;
	startTime?: string;
	endTime?: string;
	deductionMilliDays: number;
	reason: string;
	canCancel: boolean;
	createdAt: string;
	updatedAt?: string;
};

export type EmployeeLeavePayload = {
	balanceTrackingMode: EmployeeLeaveBalanceTrackingMode;
	leaveTypes: EmployeeLeaveType[];
	summary: EmployeeLeaveSummary;
	requests: EmployeeLeaveRequest[];
};

export type EmployeeLeavePreviewRequest = {
	leaveTypeID: string;
	unit: EmployeeLeaveUnit;
	startDate: string;
	endDate?: string;
	partialPeriod?: EmployeeLeavePartialPeriod;
	startTime?: string;
};

export type EmployeeLeavePreviewOccurrence = {
	date: string;
	startTime: string;
	endTime: string;
	deductionMilliDays: number;
};

export type EmployeeLeavePreviewExcludedDate = {
	date: string;
	reason: string;
};

export type EmployeeLeavePreview = {
	occurrences: EmployeeLeavePreviewOccurrence[];
	excludedDates: EmployeeLeavePreviewExcludedDate[];
	totalDeductionMilliDays: number;
};

export type EmployeeLeaveSubmission = EmployeeLeavePreviewRequest & {
	reason: string;
};


