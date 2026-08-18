import type {
	EmployeeLeaveBalanceTrackingMode,
	EmployeeLeaveSummary
} from '../../routes/attendance/leave/employee-leave-types';

type SupabaseLeaveStatus = 'requested' | 'approved' | 'rejected';

export type SupabaseLeaveSummaryRow = {
	days: number;
	status: SupabaseLeaveStatus;
	isDeducted: boolean;
	localStartDate: string;
	cancelled?: boolean;
};

export type SupabaseLeaveBalance = {
	trackingMode: EmployeeLeaveBalanceTrackingMode;
	summary: EmployeeLeaveSummary;
};

export function summarizeSupabaseLeave(
	rows: readonly SupabaseLeaveSummaryRow[],
	targetYear: number,
	remainingDays: number | null
): SupabaseLeaveBalance {
	let usedMilliDays = 0;
	let reservedMilliDays = 0;

	for (const row of rows) {
		if (row.cancelled || !row.isDeducted || Number(row.localStartDate.slice(0, 4)) !== targetYear) continue;
		const milliDays = Math.round(row.days * 1000);
		if (row.status === 'approved') usedMilliDays += milliDays;
		if (row.status === 'requested') reservedMilliDays += milliDays;
	}

	return {
		trackingMode: remainingDays === null ? 'unlimited' : 'managed',
		summary: {
			usedMilliDays,
			reservedMilliDays,
			availableMilliDays:
				remainingDays === null ? 0 : Math.round(remainingDays * 1000) - reservedMilliDays
		}
	};
}
