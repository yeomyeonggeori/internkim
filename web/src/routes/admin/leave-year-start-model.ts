export type LeaveYearStart = { month: number; day: number };

const yearTheRecordCounts = 2001;

export function daysInLeaveYearStartMonth(month: number): number {
	return new Date(yearTheRecordCounts, within(month, 1, 12), 0).getDate();
}

export function leaveYearStartWithin(month: number, day: number): LeaveYearStart {
	const chosenMonth = within(month, 1, 12);
	return { month: chosenMonth, day: within(day, 1, daysInLeaveYearStartMonth(chosenMonth)) };
}

export function sameLeaveYearStart(left: LeaveYearStart, right: LeaveYearStart): boolean {
	return left.month === right.month && left.day === right.day;
}

function within(value: number, lowest: number, highest: number): number {
	if (!Number.isFinite(value)) return lowest;
	return Math.min(highest, Math.max(lowest, Math.round(value)));
}
