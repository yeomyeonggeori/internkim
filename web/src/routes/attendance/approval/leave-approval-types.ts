export type LeaveApprovalAction = 'approve' | 'needsChanges' | 'reject';
export type LeaveApprovalStatus =
	| 'pending'
	| 'needsChanges'
	| 'approved'
	| 'rejected'
	| 'cancelled';
export type LeaveApprovalChangeKind = Exclude<LeaveApprovalStatus, 'pending'> | 'earlyReturn';

export type LeaveApprovalAttachment = {
	id: string;
	fileName: string;
	contentType: string;
	sizeBytes: number;
	downloadURL: string;
};

export type LeaveApprovalBalance = {
	availableMilliDays: number;
	reservedMilliDays: number;
	usedMilliDays: number;
};

export type LeaveApprovalRequest = {
	id: string;
	employeeEmail: string;
	leaveTypeID: string;
	leaveTypeName: string;
	balanceMode: 'annual' | 'separate' | 'none';
	status: LeaveApprovalStatus;
	unit: 'fullDay' | 'halfDay' | 'quarterDay';
	startDate: string;
	endDate?: string;
	partialPeriod?: 'morning' | 'afternoon' | 'custom';
	startTime?: string;
	endTime?: string;
	deductionMilliDays: number;
	reason: string;
	adminResponse?: string;
	attachments: LeaveApprovalAttachment[];
	balance: LeaveApprovalBalance;
	createdAt: string;
	updatedAt: string;
};

export type LeaveApprovalChange = {
	request: LeaveApprovalRequest;
	change: LeaveApprovalChangeKind;
	response?: string;
	returnedAt?: string;
	changedAt: string;
};

export type LeaveApprovalInbox = {
	pendingCount: number;
	pending: LeaveApprovalRequest[];
	recentChanges: LeaveApprovalChange[];
};

export type LeaveApprovalDecision = {
	action: LeaveApprovalAction;
	response?: string;
};
