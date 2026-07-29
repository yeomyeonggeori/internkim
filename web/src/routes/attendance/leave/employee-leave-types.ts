export type EmployeeLeaveBalanceMode = 'annual' | 'separate' | 'none';
export type EmployeeLeaveUnit = 'fullDay' | 'halfDay' | 'quarterDay';
export type EmployeeLeaveStatus =
	| 'pending'
	| 'needsChanges'
	| 'approved'
	| 'rejected'
	| 'cancelled';
export type EmployeeLeavePartialPeriod = 'morning' | 'afternoon' | 'custom';
export const employeeLeaveErrorCodes = [
	'invalidInput',
	'leaveConflict',
	'workConflict',
	'insufficientBalance',
	'hireDateRequired',
	'invalidAttachment',
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
	isActive: boolean;
	requiresHireDate: boolean;
};

export type EmployeeLeaveSummary = {
	usedMilliDays: number;
	reservedMilliDays: number;
	availableMilliDays: number;
};

export type EmployeeLeaveAttachment = {
	id: string;
	fileName: string;
	contentType: string;
	sizeBytes: number;
	downloadURL: string;
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
	adminResponse?: string;
	attachments: EmployeeLeaveAttachment[];
	canCancel: boolean;
	canEdit: boolean;
	canResubmit: boolean;
	revision: number;
	createdAt: string;
	updatedAt?: string;
};

export type EmployeeLeaveLedgerEntry = {
	id: string;
	operationKey: string;
	operationType: string;
	occurredAt: string;
	leaveTypeID: string;
	leaveTypeName: string;
	deltaMilliDays: number;
	balanceAfterMilliDays: number;
	isUntracked: boolean;
	requestID?: string;
};

export type EmployeeLeavePayload = {
	leaveTypes: EmployeeLeaveType[];
	summary: EmployeeLeaveSummary;
	requests: EmployeeLeaveRequest[];
	ledgerEntries: EmployeeLeaveLedgerEntry[];
	hireDateRequired: boolean;
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

export type EmployeeLeaveResubmission = EmployeeLeaveSubmission & {
	response?: string;
};

export type EmployeeLeaveUpdate = EmployeeLeaveSubmission & {
	revision: number;
	removedAttachmentIDs: string[];
};
