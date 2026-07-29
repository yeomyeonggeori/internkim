import type { DevEmployeeLeaveMockState } from './dev-attendance-leave-mock';
import type {
	LeaveManagementAdjustment,
	LeaveManagementEmployee,
	LeaveManagementLedgerEntry,
	LeaveManagementPastLeave,
	LeaveManagementPayload,
	LeaveManagementTimeCorrection
} from './src/routes/attendance/management/leave-management-types';

type DevLeaveManagementRequest = {
	method: string;
	pathname: string;
	searchParams: URLSearchParams;
	body?: string;
};

type DevLeaveManagementResponse = {
	status: number;
	body: unknown;
};

export type DevLeaveManagementMockState = {
	leave: DevEmployeeLeaveMockState;
	employees: LeaveManagementEmployee[];
	ledgerByEmail: Record<string, LeaveManagementLedgerEntry[]>;
};

export function createDevLeaveManagementMockState(
	leave: DevEmployeeLeaveMockState
): DevLeaveManagementMockState {
	return {
		leave,
		employees: [
			employee('kim@example.com', '김철수', 15000, 1000, 500, 13500, 13500),
			employee('seohee@example.com', '이서희', 15000, 3500, 0, 11500, 11500),
			employee('park@example.com', '박민준', 15000, 7500, 500, 7000, 7000)
		],
		ledgerByEmail: {
			'kim@example.com': [
				ledger('kim-use', 'use', -1000, 13500, '2026-07-10', '연차 사용'),
				ledger('kim-reserve', 'reserve', -500, 13500, '2026-07-29', '승인 대기'),
				ledger('kim-grant', 'legalCorrection', 15000, 15000, '2026-01-01', '2026년 연차 부여')
			],
			'seohee@example.com': [
				ledger('seohee-use', 'use', -3500, 11500, '2026-06-17', '휴가 사용'),
				ledger('seohee-grant', 'grant', 15000, 15000, '2026-01-01', '2026년 연차 부여')
			],
			'park@example.com': [
				ledger('park-use', 'use', -7500, 7000, '2026-07-22', '휴가 사용'),
				ledger('park-reserve', 'reserve', -500, 7000, '2026-08-03', '승인 대기'),
				ledger('park-grant', 'grant', 15000, 15000, '2026-01-01', '2026년 연차 부여')
			]
		}
	};
}

export function createDevLeaveManagementMockResponse(
	state: DevLeaveManagementMockState,
	request: DevLeaveManagementRequest
): DevLeaveManagementResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/attendance/api/leave-management') {
		return {
			status: 200,
			body: managementPayload(state, request.searchParams.get('email') ?? '')
		};
	}
	if (
		request.method === 'POST' &&
		request.pathname === '/attendance/api/leave-management/adjustments'
	) {
		const input = parseBody<LeaveManagementAdjustment>(request.body);
		if (!input?.employeeEmail || !input.leaveTypeID || !input.amountMilliDays) {
			return errorResponse('employee, leave type, and amount are required');
		}
		const employeeRecord = state.employees.find(
			(employeeValue) => employeeValue.email === input.employeeEmail
		);
		if (!employeeRecord) return errorResponse('employee not found', 404);
		applyAdjustment(employeeRecord, input.amountMilliDays);
		synchronizeEmployeeLeaveSummary(state, employeeRecord);
		state.ledgerByEmail[input.employeeEmail] = [
			{
				id: `adjustment-${Date.now()}`,
				operationType: input.kind,
				leaveTypeID: input.leaveTypeID,
				leaveTypeName: leaveTypeName(state, input.leaveTypeID),
				deltaMilliDays: input.amountMilliDays,
				balanceAfterMilliDays: employeeRecord.availableMilliDays,
				effectiveOn: input.effectiveOn,
				occurredAt: new Date().toISOString(),
				reason: input.reason
			},
			...(state.ledgerByEmail[input.employeeEmail] ?? [])
		];
		return { status: 200, body: { ok: true } };
	}
	if (
		request.method === 'POST' &&
		request.pathname.startsWith('/attendance/api/leave-management/requests/') &&
		request.pathname.endsWith('/time')
	) {
		const input = parseBody<LeaveManagementTimeCorrection>(request.body);
		if (!input?.employeeEmail || !input.startTime || !input.endTime || !input.reason) {
			return errorResponse('employee, start time, end time, and reason are required');
		}
		const requestID = request.pathname
			.slice('/attendance/api/leave-management/requests/'.length)
			.slice(0, -'/time'.length);
		const leaveRequest = state.leave.payload.requests.find(
			(requestValue) => requestValue.id === requestID
		);
		if (!leaveRequest || leaveRequest.status !== 'approved' || leaveRequest.unit === 'fullDay') {
			return errorResponse('approved partial leave request not found', 404);
		}
		leaveRequest.partialPeriod = 'custom';
		leaveRequest.startTime = input.startTime;
		leaveRequest.endTime = input.endTime;
		leaveRequest.updatedAt = new Date().toISOString();
		return { status: 200, body: { ok: true } };
	}
	if (
		request.method === 'POST' &&
		request.pathname === '/attendance/api/leave-management/past-leaves'
	) {
		const input = parseBody<LeaveManagementPastLeave>(request.body);
		if (!input?.employeeEmail || !input.leaveTypeID || !input.startDate) {
			return errorResponse('employee, leave type, and date are required');
		}
		const employeeRecord = state.employees.find(
			(employeeValue) => employeeValue.email === input.employeeEmail
		);
		if (!employeeRecord) return errorResponse('employee not found', 404);
		const deduction = input.unit === 'fullDay' ? 1000 : input.unit === 'halfDay' ? 500 : 250;
		const requestID = `managed-past-leave-${Date.now()}`;
		applyPastLeave(employeeRecord, deduction);
		synchronizeEmployeeLeaveSummary(state, employeeRecord);
		if (input.employeeEmail === 'kim@example.com') {
			state.leave.payload.requests = [
				{
					id: requestID,
					leaveTypeID: input.leaveTypeID,
					leaveTypeName: leaveTypeName(state, input.leaveTypeID),
					status: 'approved',
					unit: input.unit,
					startDate: input.startDate,
					endDate: input.endDate,
					startTime: input.startTime,
					deductionMilliDays: deduction,
					reason: input.reason,
					attachments: [],
					canCancel: false,
					canEdit: false,
					canResubmit: false,
					revision: 1,
					createdAt: new Date().toISOString()
				},
				...state.leave.payload.requests
			];
		}
		state.ledgerByEmail[input.employeeEmail] = [
			{
				id: `past-leave-${requestID}`,
				operationType: 'use',
				leaveTypeID: input.leaveTypeID,
				leaveTypeName: leaveTypeName(state, input.leaveTypeID),
				deltaMilliDays: -deduction,
				balanceAfterMilliDays: employeeRecord.availableMilliDays,
				effectiveOn: input.startDate,
				occurredAt: new Date().toISOString(),
				reason: input.reason
			},
			...(state.ledgerByEmail[input.employeeEmail] ?? [])
		];
		return { status: 200, body: { ok: true } };
	}
	if (
		request.method === 'POST' &&
		request.pathname.startsWith('/attendance/api/leave-management/requests/') &&
		request.pathname.endsWith('/cancel')
	) {
		const input = parseBody<{ employeeEmail?: string }>(request.body);
		if (!input?.employeeEmail) return errorResponse('employee is required');
		const requestID = request.pathname
			.slice('/attendance/api/leave-management/requests/'.length)
			.slice(0, -'/cancel'.length);
		const leaveRequest = state.leave.payload.requests.find(
			(requestValue) => requestValue.id === requestID
		);
		const employeeRecord = state.employees.find(
			(employeeValue) => employeeValue.email === input.employeeEmail
		);
		if (!leaveRequest || leaveRequest.status !== 'approved' || !employeeRecord) {
			return errorResponse('approved leave request not found', 404);
		}
		leaveRequest.status = 'cancelled';
		leaveRequest.canCancel = false;
		restorePastLeave(employeeRecord, leaveRequest.deductionMilliDays);
		synchronizeEmployeeLeaveSummary(state, employeeRecord);
		state.ledgerByEmail[input.employeeEmail] = [
			{
				id: `cancel-${requestID}`,
				operationType: 'release',
				leaveTypeID: leaveRequest.leaveTypeID,
				leaveTypeName: leaveRequest.leaveTypeName,
				deltaMilliDays: leaveRequest.deductionMilliDays,
				balanceAfterMilliDays: employeeRecord.availableMilliDays,
				effectiveOn: leaveRequest.startDate,
				occurredAt: new Date().toISOString(),
				reason: '승인된 휴가 취소'
			},
			...(state.ledgerByEmail[input.employeeEmail] ?? [])
		];
		return { status: 200, body: { ok: true } };
	}
	return undefined;
}

function managementPayload(
	state: DevLeaveManagementMockState,
	selectedEmail: string
): LeaveManagementPayload {
	const selectedEmployee = state.employees.find(
		(employeeValue) => employeeValue.email === selectedEmail
	);
	return {
		leaveTypes: structuredClone(state.leave.payload.leaveTypes),
		employees: structuredClone(state.employees),
		detail: selectedEmployee
			? {
					employee: structuredClone(selectedEmployee),
					requests:
						selectedEmail === 'kim@example.com'
							? structuredClone(state.leave.payload.requests)
							: [],
					ledgerEntries: structuredClone(state.ledgerByEmail[selectedEmail] ?? [])
				}
			: undefined
	};
}

function employee(
	email: string,
	displayName: string,
	grantedMilliDays: number,
	usedMilliDays: number,
	reservedMilliDays: number,
	availableMilliDays: number,
	expiringMilliDays: number
): LeaveManagementEmployee {
	return {
		email,
		displayName,
		grantedMilliDays,
		usedMilliDays,
		reservedMilliDays,
		availableMilliDays,
		expiringMilliDays,
		balances: [
			{
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				grantedMilliDays,
				usedMilliDays,
				reservedMilliDays,
				availableMilliDays,
				expiredMilliDays: 0,
				nextExpiryDate: '2026-12-31',
				nextExpiryMilliDays: expiringMilliDays
			}
		]
	};
}

function ledger(
	id: string,
	operationType: string,
	deltaMilliDays: number,
	balanceAfterMilliDays: number,
	effectiveOn: string,
	reason: string
): LeaveManagementLedgerEntry {
	return {
		id,
		operationType,
		leaveTypeID: 'annual',
		leaveTypeName: '연차',
		deltaMilliDays,
		balanceAfterMilliDays,
		effectiveOn,
		occurredAt: `${effectiveOn}T09:00:00+09:00`,
		reason
	};
}

function applyAdjustment(employeeRecord: LeaveManagementEmployee, amountMilliDays: number): void {
	employeeRecord.grantedMilliDays += Math.max(0, amountMilliDays);
	employeeRecord.availableMilliDays += amountMilliDays;
	employeeRecord.expiringMilliDays += Math.max(0, amountMilliDays);
	const balance = employeeRecord.balances[0];
	if (!balance) return;
	balance.grantedMilliDays += Math.max(0, amountMilliDays);
	balance.availableMilliDays += amountMilliDays;
	balance.nextExpiryMilliDays += Math.max(0, amountMilliDays);
}

function applyPastLeave(employeeRecord: LeaveManagementEmployee, deduction: number): void {
	employeeRecord.availableMilliDays -= deduction;
	employeeRecord.usedMilliDays += deduction;
	const balance = employeeRecord.balances[0];
	if (!balance) return;
	balance.availableMilliDays -= deduction;
	balance.usedMilliDays += deduction;
}

function restorePastLeave(employeeRecord: LeaveManagementEmployee, deduction: number): void {
	employeeRecord.availableMilliDays += deduction;
	employeeRecord.usedMilliDays -= deduction;
	const balance = employeeRecord.balances[0];
	if (!balance) return;
	balance.availableMilliDays += deduction;
	balance.usedMilliDays -= deduction;
}

function synchronizeEmployeeLeaveSummary(
	state: DevLeaveManagementMockState,
	employeeRecord: LeaveManagementEmployee
): void {
	if (employeeRecord.email !== 'kim@example.com') return;
	state.leave.payload.summary.availableMilliDays = employeeRecord.availableMilliDays;
	state.leave.payload.summary.reservedMilliDays = employeeRecord.reservedMilliDays;
	state.leave.payload.summary.usedMilliDays = employeeRecord.usedMilliDays;
}

function leaveTypeName(state: DevLeaveManagementMockState, leaveTypeID: string): string {
	return (
		state.leave.payload.leaveTypes.find((leaveType) => leaveType.id === leaveTypeID)?.name ??
		leaveTypeID
	);
}

function parseBody<Value>(body: string | undefined): Value | null {
	try {
		return JSON.parse(body ?? '{}') as Value;
	} catch {
		return null;
	}
}

function errorResponse(error: string, status = 400): DevLeaveManagementResponse {
	return { status, body: { code: 'invalidInput', error } };
}
