import type {
	EmployeeLeavePayload,
	EmployeeLeaveSummary,
	EmployeeLeaveType
} from './src/routes/attendance/leave/employee-leave-types';

type EmployeeLeaveBalanceMutation = Partial<EmployeeLeaveSummary>;

export function applyEmployeeLeaveBalanceMutation(
	payload: EmployeeLeavePayload,
	leaveTypeID: string,
	mutation: EmployeeLeaveBalanceMutation
): EmployeeLeaveSummary | undefined {
	if (payload.balanceTrackingMode === 'unlimited') return undefined;
	const requestedType = payload.leaveTypes.find((leaveType) => leaveType.id === leaveTypeID);
	const owner = balanceOwner(payload.leaveTypes, requestedType);
	if (!owner?.balance) return undefined;

	for (const leaveType of accountLeaveTypes(payload.leaveTypes, owner)) {
		if (!leaveType.balance) continue;
		applyMutation(leaveType.balance, mutation);
	}
	if (owner.includeInSummary) {
		applyMutation(payload.summary, mutation);
	}
	return structuredClone(owner.balance);
}

export function synchronizeUnlimitedEmployeeLeaveUsage(payload: EmployeeLeavePayload): void {
	if (payload.balanceTrackingMode !== 'unlimited') return;
	const usageByLeaveType = new Map<string, EmployeeLeaveSummary>();
	for (const leaveType of payload.leaveTypes) {
		usageByLeaveType.set(leaveType.id, {
			usedMilliDays: 0,
			reservedMilliDays: 0,
			availableMilliDays: 0
		});
	}
	for (const request of payload.requests) {
		const usage = usageByLeaveType.get(request.leaveTypeID);
		if (!usage) continue;
		if (request.status === 'approved') {
			usage.usedMilliDays += request.deductionMilliDays;
		}
		if (request.status === 'pending' || request.status === 'needsChanges') {
			usage.reservedMilliDays += request.deductionMilliDays;
		}
	}
	payload.summary = {
		usedMilliDays: 0,
		reservedMilliDays: 0,
		availableMilliDays: 0
	};
	for (const leaveType of payload.leaveTypes) {
		const usage = usageByLeaveType.get(leaveType.id);
		leaveType.balance = usage ? structuredClone(usage) : undefined;
		leaveType.requiresHireDate = false;
		payload.summary.usedMilliDays += usage?.usedMilliDays ?? 0;
		payload.summary.reservedMilliDays += usage?.reservedMilliDays ?? 0;
	}
}

export function employeeLeaveTypeBalance(
	payload: EmployeeLeavePayload,
	leaveTypeID: string
): EmployeeLeaveSummary | undefined {
	const requestedType = payload.leaveTypes.find((leaveType) => leaveType.id === leaveTypeID);
	const owner = balanceOwner(payload.leaveTypes, requestedType);
	return owner?.balance ? structuredClone(owner.balance) : undefined;
}

function balanceOwner(
	leaveTypes: EmployeeLeaveType[],
	requestedType: EmployeeLeaveType | undefined
): EmployeeLeaveType | undefined {
	if (!requestedType || requestedType.balanceMode === 'none') return undefined;
	if (requestedType.balanceMode === 'annual') {
		return leaveTypes.find((leaveType) => leaveType.id === 'annual') ?? requestedType;
	}
	return requestedType;
}

function accountLeaveTypes(
	leaveTypes: EmployeeLeaveType[],
	owner: EmployeeLeaveType
): EmployeeLeaveType[] {
	if (owner.balanceMode === 'annual') {
		return leaveTypes.filter((leaveType) => leaveType.balanceMode === 'annual');
	}
	return [owner];
}

function applyMutation(
	balance: EmployeeLeaveSummary,
	mutation: EmployeeLeaveBalanceMutation
): void {
	balance.availableMilliDays += mutation.availableMilliDays ?? 0;
	balance.reservedMilliDays += mutation.reservedMilliDays ?? 0;
	balance.usedMilliDays += mutation.usedMilliDays ?? 0;
}
