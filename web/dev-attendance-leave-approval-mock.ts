import type { DevEmployeeLeaveMockState } from './dev-attendance-leave-mock';
import type { EmployeeLeaveRequest } from './src/routes/attendance/leave/employee-leave-types';
import type {
	LeaveApprovalAction,
	LeaveApprovalChange,
	LeaveApprovalInbox,
	LeaveApprovalRequest,
	LeaveApprovalStatus
} from './src/routes/attendance/approval/leave-approval-types';
import {
	applyEmployeeLeaveBalanceMutation,
	employeeLeaveTypeBalance
} from './dev-attendance-leave-balance';

type DevLeaveApprovalRequest = {
	method: string;
	pathname: string;
	body?: string;
};

type DevLeaveApprovalResponse = {
	status: number;
	body: unknown;
};

export type DevLeaveApprovalMockState = {
	leave: DevEmployeeLeaveMockState;
	recentChanges: LeaveApprovalChange[];
};

export function createDevLeaveApprovalMockState(
	leave: DevEmployeeLeaveMockState
): DevLeaveApprovalMockState {
	const recentChanges = leave.payload.requests
		.filter((request) => request.status !== 'pending')
		.map((request) => ({
			request: projectLeaveApprovalRequest(leave, request),
			change: request.status as Exclude<LeaveApprovalStatus, 'pending'>,
			response: request.adminResponse,
			changedAt: request.updatedAt ?? request.createdAt
		}))
		.sort((first, second) => second.changedAt.localeCompare(first.changedAt));
	return { leave, recentChanges };
}

export function createDevLeaveApprovalMockResponse(
	state: DevLeaveApprovalMockState,
	request: DevLeaveApprovalRequest
): DevLeaveApprovalResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/attendance/api/leave-approvals') {
		return { status: 200, body: buildLeaveApprovalInbox(state) };
	}
	const decisionMatch = request.pathname.match(/^\/attendance\/api\/leave-approvals\/([^/]+)$/);
	if (request.method !== 'POST' || !decisionMatch) return undefined;
	const requestID = decodeURIComponent(decisionMatch[1] ?? '');
	const employeeRequest = state.leave.payload.requests.find(
		(candidate) => candidate.id === requestID
	);
	if (!employeeRequest) {
		return errorResponse(404, 'requestNotFound', 'leave request not found');
	}
	if (employeeRequest.status !== 'pending') {
		return errorResponse(409, 'invalidStatus', 'leave request is no longer pending');
	}
	const decision = decisionFromBody(request.body);
	if (!decision) {
		return errorResponse(400, 'invalidInput', 'unsupported leave approval action');
	}
	if (decision.action === 'needsChanges' && !decision.response) {
		return errorResponse(400, 'invalidInput', 'response is required when requesting changes');
	}
	applyDecision(state, employeeRequest, decision.action, decision.response);
	const approvalRequest = projectLeaveApprovalRequest(state.leave, employeeRequest);
	state.recentChanges = [
		{
			request: approvalRequest,
			change: employeeRequest.status as Exclude<LeaveApprovalStatus, 'pending'>,
			response: decision.response || undefined,
			changedAt: employeeRequest.updatedAt ?? employeeRequest.createdAt
		},
		...state.recentChanges
	];
	return { status: 200, body: { request: approvalRequest } };
}

function buildLeaveApprovalInbox(state: DevLeaveApprovalMockState): LeaveApprovalInbox {
	const pending = state.leave.payload.requests
		.filter((request) => request.status === 'pending')
		.map((request) => projectLeaveApprovalRequest(state.leave, request));
	const currentRequestChanges = state.leave.payload.requests
		.filter((request) => request.status !== 'pending')
		.map((request) => ({
			request: projectLeaveApprovalRequest(state.leave, request),
			change: request.status as Exclude<LeaveApprovalStatus, 'pending'>,
			response: request.adminResponse,
			changedAt: request.updatedAt ?? request.createdAt
		}));
	const earlyReturnChanges = state.recentChanges.filter(
		(change) => change.change === 'earlyReturn'
	);
	return {
		pendingCount: pending.length,
		pending,
		recentChanges: structuredClone(
			[...earlyReturnChanges, ...currentRequestChanges].sort((first, second) =>
				second.changedAt.localeCompare(first.changedAt)
			)
		)
	};
}

function projectLeaveApprovalRequest(
	state: DevEmployeeLeaveMockState,
	request: EmployeeLeaveRequest
): LeaveApprovalRequest {
	const leaveType = state.payload.leaveTypes.find(
		(candidate) => candidate.id === request.leaveTypeID
	);
	return {
		id: request.id,
		employeeEmail: 'kim@example.com',
		leaveTypeID: request.leaveTypeID,
		leaveTypeName: request.leaveTypeName,
		balanceMode: leaveType?.balanceMode ?? 'none',
		status: request.status,
		unit: request.unit,
		startDate: request.startDate,
		endDate: request.endDate,
		partialPeriod: request.partialPeriod,
		startTime: request.startTime,
		endTime: request.endTime,
		deductionMilliDays: request.deductionMilliDays,
		reason: request.reason,
		adminResponse: request.adminResponse,
		attachments: structuredClone(request.attachments),
		balance: employeeLeaveTypeBalance(state.payload, request.leaveTypeID) ?? {
			availableMilliDays: 0,
			reservedMilliDays: 0,
			usedMilliDays: 0
		},
		createdAt: request.createdAt,
		updatedAt: request.updatedAt ?? request.createdAt
	};
}

function applyDecision(
	state: DevLeaveApprovalMockState,
	request: EmployeeLeaveRequest,
	action: LeaveApprovalAction,
	response: string
): void {
	const nextStatus = approvalStatus(action);
	request.status = nextStatus;
	request.adminResponse = response || undefined;
	request.updatedAt = new Date().toISOString();
	request.canCancel = nextStatus === 'needsChanges' || nextStatus === 'approved';
	request.canEdit = false;
	request.canResubmit = nextStatus === 'needsChanges';
	request.revision += 1;
	if (action === 'approve') {
		applyEmployeeLeaveBalanceMutation(state.leave.payload, request.leaveTypeID, {
			reservedMilliDays: -request.deductionMilliDays,
			usedMilliDays: request.deductionMilliDays
		});
	}
	if (action === 'reject') {
		applyEmployeeLeaveBalanceMutation(state.leave.payload, request.leaveTypeID, {
			availableMilliDays: request.deductionMilliDays,
			reservedMilliDays: -request.deductionMilliDays
		});
	}
}

function approvalStatus(action: LeaveApprovalAction): LeaveApprovalStatus {
	switch (action) {
		case 'approve':
			return 'approved';
		case 'reject':
			return 'rejected';
		default:
			return 'needsChanges';
	}
}

function decisionFromBody(
	body: string | undefined
): { action: LeaveApprovalAction; response: string } | null {
	try {
		const parsed = JSON.parse(body ?? '{}') as Record<string, unknown>;
		const action = parsed.action;
		if (action !== 'approve' && action !== 'needsChanges' && action !== 'reject') return null;
		return {
			action,
			response: typeof parsed.response === 'string' ? parsed.response.trim() : ''
		};
	} catch {
		return null;
	}
}

function errorResponse(status: number, code: string, error: string): DevLeaveApprovalResponse {
	return { status, body: { code, error } };
}
