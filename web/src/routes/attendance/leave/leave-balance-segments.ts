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

export function leaveBalanceSegments(
	summary: EmployeeLeaveSummary | undefined,
	pendingIsInsideRemaining: boolean
): LeaveBalanceSegments {
	const used = Math.max(summary?.usedMilliDays ?? 0, 0);
	const remaining = Math.max(summary?.availableMilliDays ?? 0, 0);
	const pending = Math.max(summary?.reservedMilliDays ?? 0, 0);

	if (!pendingIsInsideRemaining) {
		const grant = used + pending + remaining;
		if (grant <= 0) return nothingDrawn;
		return {
			usedPercent: (used / grant) * 100,
			reservedPercent: (pending / grant) * 100,
			availablePercent: (remaining / grant) * 100
		};
	}

	const grant = used + remaining;
	if (grant <= 0) return nothingDrawn;
	const claimed = Math.min(pending, remaining);
	return {
		usedPercent: (used / grant) * 100,
		reservedPercent: (claimed / grant) * 100,
		availablePercent: ((remaining - claimed) / grant) * 100
	};
}
