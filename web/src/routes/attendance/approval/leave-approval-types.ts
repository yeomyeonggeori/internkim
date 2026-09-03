export type LeaveApprovalAction = 'approve' | 'reject';
export type LeaveApprovalStatus = 'pending' | 'approved' | 'rejected' | 'cancelled';

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
	createdAt: string;
	updatedAt: string;
};

export type LeaveApprovalInbox = {
	pendingCount: number;
	pending: LeaveApprovalRequest[];
};

export type LeaveApprovalDecision = {
	action: LeaveApprovalAction;
};
