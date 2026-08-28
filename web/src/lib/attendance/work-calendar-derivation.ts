import type { CurrentAttendanceWorkPolicy } from './current-work-policy';
import type { AttendanceWorkMode } from '$lib/attendance/work-mode';

export const initialEffectiveDate = '1970-01-01';

export type AttendanceWorkPolicyRevision = CurrentAttendanceWorkPolicy & {
	effectiveDate: string;
};

export function companyWeekday(day: string): number {
	const weekday = new Date(`${day}T00:00:00Z`).getUTCDay();
	return weekday === 0 ? 7 : weekday;
}

export function revisionForDate(
	revisions: AttendanceWorkPolicyRevision[],
	day: string
): AttendanceWorkPolicyRevision {
	if (revisions.length === 0) throw new Error('a work policy needs at least one revision');
	let inForce = revisions[0];
	for (const revision of revisions) {
		if (revision.effectiveDate > day) break;
		inForce = revision;
	}
	return inForce;
}

export function workModeForDate(
	revisions: AttendanceWorkPolicyRevision[],
	day: string
): AttendanceWorkMode {
	return revisionForDate(revisions, day).workMode;
}

export function workingDateForDate(
	revision: AttendanceWorkPolicyRevision,
	day: string,
	holidays: ReadonlySet<string>
): boolean {
	if (holidays.has(day)) return false;
	return revision.workingWeekdays.includes(companyWeekday(day));
}
