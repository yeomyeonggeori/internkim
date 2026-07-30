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
