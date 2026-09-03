import type { EmployeeLeaveSummary } from './employee-leave-types';

export type LeaveBalanceSegments = {
	usedPercent: number;
	reservedPercent: number;
	availablePercent: number;
};

const nothingDrawn: LeaveBalanceSegments = {
	usedPercent: 0,
	reservedPercent: 0,
	availablePercent: 0
};

// The record counts a pending leave inside what remains, so the bar is the
// grant that was used plus the grant that is left, and pending is drawn
// within the remaining part rather than beside it.
export function leaveBalanceSegments(
	summary: EmployeeLeaveSummary | undefined
): LeaveBalanceSegments {
	const used = Math.max(summary?.usedMilliDays ?? 0, 0);
	const remaining = Math.max(summary?.availableMilliDays ?? 0, 0);
	const pending = Math.max(summary?.reservedMilliDays ?? 0, 0);

	const grant = used + remaining;
	if (grant <= 0) return nothingDrawn;
	const claimed = Math.min(pending, remaining);
	return {
		usedPercent: (used / grant) * 100,
		reservedPercent: (claimed / grant) * 100,
		availablePercent: ((remaining - claimed) / grant) * 100
	};
}
