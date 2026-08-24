import type { EmployeeLeaveSummary } from './employee-leave-types';

export type LeaveBalanceSegments = {
	usedPercent: number;
	reservedPercent: number;
	availablePercent: number;
};

export function leaveBalanceSegments(
	summary: EmployeeLeaveSummary | undefined
): LeaveBalanceSegments {
	const used = Math.max(summary?.usedMilliDays ?? 0, 0);
	const available = Math.max(summary?.availableMilliDays ?? 0, 0);
	const total = used + available;
	if (total <= 0) return { usedPercent: 0, reservedPercent: 0, availablePercent: 0 };

	const pending = Math.min(Math.max(summary?.reservedMilliDays ?? 0, 0), available);
	return {
		usedPercent: (used / total) * 100,
		reservedPercent: (pending / total) * 100,
		availablePercent: ((available - pending) / total) * 100
	};
}
