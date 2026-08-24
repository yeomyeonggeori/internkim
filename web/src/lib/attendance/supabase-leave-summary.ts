import type {
	EmployeeLeaveBalanceTrackingMode,
	EmployeeLeaveSummary
} from '../../routes/attendance/leave/employee-leave-types';
import { leaveDaysInYear } from './leave-year-share';

type SupabaseLeaveStatus = 'requested' | 'approved' | 'rejected';

export type SupabaseLeaveSummaryRow = {
	days: number;
	status: SupabaseLeaveStatus;
	isDeducted: boolean;
	localStartDate: string;
	localEndDate: string;
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
		if (!row.isDeducted) continue;
		const days = leaveDaysInYear(row.days, row.localStartDate, row.localEndDate, targetYear);
		if (days === 0) continue;
		const milliDays = Math.round(days * 1000);
		if (row.status === 'approved') usedMilliDays += milliDays;
		if (row.status === 'requested') reservedMilliDays += milliDays;
	}

	return {
		trackingMode: remainingDays === null ? 'unlimited' : 'managed',
		summary: {
			usedMilliDays,
			reservedMilliDays,
			availableMilliDays: remainingDays === null ? 0 : Math.round(remainingDays * 1000)
		}
	};
}
