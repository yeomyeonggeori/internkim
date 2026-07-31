import type {
	AttendanceWorkBreakPeriod,
	AttendanceWorkMode,
	AttendanceWorkPolicy,
	AttendanceWorkPolicyRevision
} from './admin-types';

export function copyAttendanceWorkPolicyRevision(
	revision: AttendanceWorkPolicyRevision
): AttendanceWorkPolicyRevision {
	return {
		...revision,
		workingWeekdays: [...revision.workingWeekdays],
		breakPeriods: revision.breakPeriods.map((period) => ({ ...period }))
	};
}

export function currentAttendanceWorkPolicyRevision(
	policy: AttendanceWorkPolicy
): AttendanceWorkPolicyRevision {
	const latest = policy.revisions.reduce((current, revision) =>
		revision.effectiveDate > current.effectiveDate ? revision : current
	);
	return copyAttendanceWorkPolicyRevision(latest);
}

export function setAttendanceWorkingWeekdays(
	revision: AttendanceWorkPolicyRevision,
	workingWeekdays: number[]
): AttendanceWorkPolicyRevision {
	const normalizedWeekdays = [...new Set(workingWeekdays)].sort((left, right) => left - right);
	const next = copyAttendanceWorkPolicyRevision(revision);
	next.workingWeekdays = normalizedWeekdays;
	if (next.workMode !== 'autonomous' && normalizedWeekdays.length > 0) {
		next.dailyTargetMinutes = Math.round(next.weeklyTargetMinutes / normalizedWeekdays.length);
	}
	return next;
}

export function setAttendanceWeeklyTargetMinutes(
	revision: AttendanceWorkPolicyRevision,
	weeklyTargetMinutes: number
): AttendanceWorkPolicyRevision {
	const next = copyAttendanceWorkPolicyRevision(revision);
	next.weeklyTargetMinutes = weeklyTargetMinutes;
	if (next.workingWeekdays.length > 0) {
		next.dailyTargetMinutes = Math.round(weeklyTargetMinutes / next.workingWeekdays.length);
	}
	return next;
}

export function fixedAttendanceTargetMinutes(
	startTime: string,
	endTime: string,
	breakPeriods: AttendanceWorkBreakPeriod[]
): number {
	const startMinute = attendanceTimeMinutes(startTime);
	const endMinute = attendanceTimeMinutes(endTime);
	if (startMinute === null || endMinute === null || startMinute >= endMinute) return 0;
	const breakMinutes = breakPeriods.reduce((total, period) => {
		const breakStart = attendanceTimeMinutes(period.startTime);
		const breakEnd = attendanceTimeMinutes(period.endTime);
		if (breakStart === null || breakEnd === null || breakStart >= breakEnd) return total;
		return total + Math.max(0, Math.min(endMinute, breakEnd) - Math.max(startMinute, breakStart));
	}, 0);
	return Math.max(0, endMinute - startMinute - breakMinutes);
}

export function attendanceMonthlyTargetMinutes(
	revision: AttendanceWorkPolicyRevision,
	month: string,
	holidayDates: string[] = []
): number {
	if (revision.workMode === 'autonomous' || !/^\d{4}-\d{2}$/.test(month)) return 0;
	const [year, monthNumber] = month.split('-').map(Number);
	if (monthNumber < 1 || monthNumber > 12) return 0;
	const workingWeekdays = new Set(revision.workingWeekdays);
	const holidays = new Set(holidayDates);
	const finalDay = new Date(Date.UTC(year, monthNumber, 0)).getUTCDate();
	let workingDayCount = 0;
	for (let day = 1; day <= finalDay; day += 1) {
		const date = new Date(Date.UTC(year, monthNumber - 1, day));
		const dateValue = date.toISOString().slice(0, 10);
		if (workingWeekdays.has(date.getUTCDay()) && !holidays.has(dateValue)) {
			workingDayCount += 1;
		}
	}
	return workingDayCount * revision.dailyTargetMinutes;
}

export function setAttendanceWorkMode(
	revision: AttendanceWorkPolicyRevision,
	workMode: AttendanceWorkMode
): AttendanceWorkPolicyRevision {
	const next = copyAttendanceWorkPolicyRevision(revision);
	next.workMode = workMode;
	if (workMode === 'autonomous') {
		next.weeklyTargetMinutes = 0;
		next.coreTimeEnabled = false;
		next.coreStartTime = '';
		next.coreEndTime = '';
		next.fixedStartTime = '';
		next.fixedEndTime = '';
		return next;
	}
	if (workMode === 'fixed') {
		next.fixedStartTime = next.fixedStartTime || '09:00';
		next.fixedEndTime = next.fixedEndTime || '18:00';
		next.coreTimeEnabled = false;
		next.coreStartTime = '';
		next.coreEndTime = '';
		next.dailyTargetMinutes = fixedAttendanceTargetMinutes(
			next.fixedStartTime,
			next.fixedEndTime,
			next.breakPeriods
		);
		next.weeklyTargetMinutes = next.dailyTargetMinutes * next.workingWeekdays.length;
		next.referenceStartTime = next.fixedStartTime;
		return next;
	}
	next.fixedStartTime = '';
	next.fixedEndTime = '';
	if (next.weeklyTargetMinutes <= 0) {
		next.weeklyTargetMinutes = next.dailyTargetMinutes * next.workingWeekdays.length;
	}
	return next;
}

function attendanceTimeMinutes(value: string): number | null {
	if (!/^\d{2}:\d{2}$/.test(value)) return null;
	const [hour, minute] = value.split(':').map(Number);
	if (hour > 23 || minute > 59) return null;
	return hour * 60 + minute;
}
