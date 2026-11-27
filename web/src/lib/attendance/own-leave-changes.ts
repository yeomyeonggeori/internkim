import type { EmployeeLeaveRequest } from '../../routes/attendance/leave/employee-leave-types';

export type OwnLeave = {
	row: { id: string; starts_at: string; original_leave_id: string | null };
	request: EmployeeLeaveRequest;
};

export function ownLeaveRequests(leave: OwnLeave[], now: Date): EmployeeLeaveRequest[] {
	const requestOf = new Map(leave.map(({ row, request }) => [row.id, request]));
	const awaitingReplacement = new Set(
		leave
			.filter(({ row, request }) => request.status === 'pending' && row.original_leave_id !== null)
			.map(({ row }) => row.original_leave_id)
	);

	return leave.map(({ row, request }) => {
		const standsUnstarted = request.status === 'approved' && new Date(row.starts_at) > now;
		const original = row.original_leave_id ? requestOf.get(row.original_leave_id) : undefined;
		return {
			...request,
			canCancel: request.status === 'pending' || standsUnstarted,
			canChange: standsUnstarted && !awaitingReplacement.has(row.id),
			...(original
				? { replaces: { id: original.id, startDate: original.startDate, endDate: original.endDate } }
				: {})
		};
	});
}
