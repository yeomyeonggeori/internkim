import type { AttendanceAbsence } from '../attendance-context.svelte';

type AbsenceLabelText = {
	absenceKindLeave: string;
	absenceKindOther: string;
};

export function absencesForDate(
	absences: AttendanceAbsence[],
	date: string,
	email: string
): AttendanceAbsence[] {
	return absences.filter((absence) => absence.date === date && absence.email === email && !absence.canceledAt);
}

export function absenceLabelText(absence: AttendanceAbsence, text: AbsenceLabelText): string {
	switch (absence.labelKey) {
		case 'leave':
			return text.absenceKindLeave;
		case 'other':
			return text.absenceKindOther;
	}
}

export function hasAbsenceDetails(absence: AttendanceAbsence): boolean {
	return Boolean(absence.reason || absence.createdBy);
}
