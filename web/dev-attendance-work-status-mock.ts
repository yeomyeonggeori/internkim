import type {
	AttendanceEmployeeWorkStatus,
	AttendanceWorkStatus,
	AttendanceWorkStatusPeriod
} from './src/routes/attendance/attendance-api';

export function createDevAttendanceWorkStatus(
	userEmail: string,
	periodValue: string | null,
	anchorValue: string | null
): AttendanceWorkStatus {
	const period = workStatusPeriod(periodValue);
	const anchor = validDate(anchorValue) ? anchorValue : '2026-07-31';
	const { start, end, workingDays } = workStatusRange(period, anchor);
	const targetMinutes = period === 'day' ? 480 : period === 'week' ? 2400 : workingDays * 480;
	const employees = [
		workStatusEmployee('김민지', 'minji@internkim.com', start, end, targetMinutes, 0, 0),
		workStatusEmployee('박지훈', 'jihoon@internkim.com', start, end, targetMinutes, 180, 35),
		workStatusEmployee('이서연', 'seoyeon@internkim.com', start, end, targetMinutes, -240, 0),
		workStatusEmployee('최도윤', 'doyoon@internkim.com', start, end, targetMinutes, 0, 70)
	];
	employees[2].coreTimeMissed = true;
	employees[2].status = 'coreTimeMissed';
	employees[3].needsReview = true;
	employees[3].hasIncompleteRecords = true;
	employees[3].status = 'needsReview';
	const personal = workStatusEmployee('관리자', userEmail, start, end, targetMinutes, 80, 25, 480);
	return {
		period,
		anchor,
		periodStart: start,
		periodEnd: end,
		timeZone: 'Asia/Seoul',
		isAdmin: true,
		personal,
		employees: [personal, ...employees]
	};
}

function workStatusEmployee(
	displayName: string,
	email: string,
	periodStart: string,
	periodEnd: string,
	targetMinutes: number,
	targetDifference: number,
	nightMinutes: number,
	leaveMinutes = 0
): AttendanceEmployeeWorkStatus {
	const adjustedTargetMinutes = Math.max(0, targetMinutes - leaveMinutes);
	const actualMinutes = Math.max(0, adjustedTargetMinutes + targetDifference);
	const fulfilledMinutes = Math.min(adjustedTargetMinutes, actualMinutes);
	const remainingMinutes = Math.max(0, adjustedTargetMinutes - fulfilledMinutes);
	const overtimeMinutes = Math.max(0, actualMinutes - adjustedTargetMinutes);
	const differenceMinutes = actualMinutes - adjustedTargetMinutes;
	const status =
		overtimeMinutes > 0 ? 'overtime' : remainingMinutes > 0 ? 'remaining' : 'fulfilled';
	return {
		email,
		displayName,
		periodStart,
		periodEnd,
		workMode: 'flexible',
		hasBaseline: true,
		targetMinutes: adjustedTargetMinutes,
		actualMinutes,
		provisionalMinutes: 0,
		leaveMinutes,
		fulfilledMinutes,
		differenceMinutes,
		remainingMinutes,
		overtimeMinutes,
		nightMinutes,
		isWorking: false,
		needsReview: false,
		coreTimeMissed: false,
		late: false,
		earlyLeave: false,
		hasLeaveWorkOverlap: false,
		hasIncompleteRecords: false,
		status,
		days: [
			{
				date: periodStart,
				workMode: 'flexible',
				hasBaseline: true,
				targetMinutes: Math.min(480, adjustedTargetMinutes),
				actualMinutes: Math.min(480, actualMinutes),
				provisionalMinutes: 0,
				leaveMinutes: Math.min(480, leaveMinutes),
				fulfilledMinutes: Math.min(480, fulfilledMinutes),
				differenceMinutes,
				remainingMinutes: Math.min(480, remainingMinutes),
				overtimeMinutes: Math.min(180, overtimeMinutes),
				nightMinutes: Math.min(180, nightMinutes),
				isWorking: false,
				needsReview: false,
				coreTimeMissed: false,
				late: false,
				earlyLeave: false,
				hasLeaveWorkOverlap: false,
				hasIncompleteWorkRecord: false,
				status,
				workSegments: [],
				leaveSegments: []
			}
		]
	};
}

function workStatusPeriod(value: string | null): AttendanceWorkStatusPeriod {
	if (value === 'day' || value === 'month') return value;
	return 'week';
}

function validDate(value: string | null): value is string {
	return Boolean(value && /^\d{4}-\d{2}-\d{2}$/.test(value));
}

function workStatusRange(
	period: AttendanceWorkStatusPeriod,
	anchor: string
): { start: string; end: string; workingDays: number } {
	const date = new Date(`${anchor}T00:00:00Z`);
	if (period === 'day') return { start: anchor, end: anchor, workingDays: 1 };
	if (period === 'month') {
		const start = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 1));
		const end = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + 1, 0));
		return {
			start: start.toISOString().slice(0, 10),
			end: end.toISOString().slice(0, 10),
			workingDays: weekdaysBetween(start, end)
		};
	}
	const weekdayOffset = (date.getUTCDay() + 6) % 7;
	const start = new Date(date);
	start.setUTCDate(start.getUTCDate() - weekdayOffset);
	const end = new Date(start);
	end.setUTCDate(end.getUTCDate() + 6);
	return {
		start: start.toISOString().slice(0, 10),
		end: end.toISOString().slice(0, 10),
		workingDays: 5
	};
}

function weekdaysBetween(start: Date, end: Date): number {
	let count = 0;
	const cursor = new Date(start);
	while (cursor <= end) {
		const weekday = cursor.getUTCDay();
		if (weekday !== 0 && weekday !== 6) count += 1;
		cursor.setUTCDate(cursor.getUTCDate() + 1);
	}
	return count;
}
