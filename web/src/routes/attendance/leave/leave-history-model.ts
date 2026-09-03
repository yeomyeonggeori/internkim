import type { EmployeeLeaveRequest } from './employee-leave-types';

export type LeaveHistoryItem = {
	id: string;
	occurredAt: string;
	request: EmployeeLeaveRequest;
};

export function buildLeaveHistory(requests: EmployeeLeaveRequest[]): LeaveHistoryItem[] {
	return requests
		.map((request) => ({
			id: `request:${request.id}`,
			occurredAt: request.updatedAt ?? request.createdAt,
			request
		}))
		.sort((first, second) => second.occurredAt.localeCompare(first.occurredAt));
}

export function milliDaysValue(milliDays: number): string {
	const days = milliDays / 1000;
	return Number.isInteger(days) ? String(days) : String(Number(days.toFixed(3)));
}
