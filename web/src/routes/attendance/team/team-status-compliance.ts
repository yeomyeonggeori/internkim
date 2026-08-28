import type { AttendanceEmployeeWorkStatus } from '../attendance-api';
import {
	workStatusCompliance,
	type WorkStatusComplianceKind
} from '../work-status/work-status-compliance';

const marksByPayload = new WeakMap<
	AttendanceEmployeeWorkStatus[],
	Map<string, WorkStatusComplianceKind[]>
>();

export function teamComplianceMarks(
	employees: AttendanceEmployeeWorkStatus[],
	email: string,
	date: string
): WorkStatusComplianceKind[] {
	let marks = marksByPayload.get(employees);
	if (!marks) {
		marks = buildMarks(employees);
		marksByPayload.set(employees, marks);
	}
	return marks.get(markKey(email, date)) ?? [];
}

function buildMarks(
	employees: AttendanceEmployeeWorkStatus[]
): Map<string, WorkStatusComplianceKind[]> {
	const marks = new Map<string, WorkStatusComplianceKind[]>();
	for (const employee of employees) {
		for (const day of employee.days) {
			const kinds = workStatusCompliance([day]).map((entry) => entry.kind);
			if (kinds.length > 0) marks.set(markKey(employee.email, day.date), kinds);
		}
	}
	return marks;
}

function markKey(email: string, date: string): string {
	return `${email} ${date}`;
}
