import type { EmployeeLeaveSummary } from './employee-leave-types';

export type LeaveBalanceSegments = {
	usedPercent: number;
	reservedPercent: number;
	availablePercent: number;
};

/**
 * Widths for the used / pending / remaining bar, as percentages of the grant.
 *
 * Remaining does not have pending taken out of it, so the bar draws pending
 * inside remaining rather than beside it: used plus remaining is the whole
 * grant, and pending shades the part of remaining that is already spoken for.
 * Adding pending as a fourth slice would draw the same days twice and stretch
 * the bar past the grant.
 */
export function leaveBalanceSegments(
	summary: EmployeeLeaveSummary | undefined
): LeaveBalanceSegments {
	const used = summary?.usedMilliDays ?? 0;
	const available = summary?.availableMilliDays ?? 0;
	const total = used + available;
	if (total <= 0) return { usedPercent: 0, reservedPercent: 0, availablePercent: 0 };

	const pending = Math.min(Math.max(summary?.reservedMilliDays ?? 0, 0), available);
	return {
		usedPercent: (used / total) * 100,
		reservedPercent: (pending / total) * 100,
		availablePercent: ((available - pending) / total) * 100
	};
}
