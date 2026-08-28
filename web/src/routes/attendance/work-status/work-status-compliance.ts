import type { AttendanceWorkDayStatus } from '../attendance-api';

export type WorkStatusComplianceKind = 'late' | 'earlyLeave' | 'coreTimeMissed';

export type WorkStatusComplianceCount = {
	kind: WorkStatusComplianceKind;
	dayCount: number;
};

const complianceKinds: WorkStatusComplianceKind[] = ['late', 'earlyLeave', 'coreTimeMissed'];

export function workStatusCompliance(
	days: AttendanceWorkDayStatus[]
): WorkStatusComplianceCount[] {
	const counts: WorkStatusComplianceCount[] = [];
	for (const kind of complianceKinds) {
		const dayCount = days.filter((day) => day[kind]).length;
		if (dayCount > 0) counts.push({ kind, dayCount });
	}
	return counts;
}
