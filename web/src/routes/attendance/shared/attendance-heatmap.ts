import type { AttendanceAbsence, AttendanceEvent } from '../attendance-context.svelte';
import { eachDayOfMonth } from './attendance-date';
import { uniquePeople } from './attendance-people';

export type DayHeatCell = {
	date: string;
	absenceCount: number;
	totalPeople: number;
};

export function computeHeatmap(month: string, events: AttendanceEvent[], absences: AttendanceAbsence[] = []): DayHeatCell[] {
	const days = eachDayOfMonth(month);
	const monthEvents = events.filter((event) => event.localDate.startsWith(month));
	const monthAbsences = absences.filter((absence) => absence.date.startsWith(month) && !absence.canceledAt);
	const peopleCount = uniquePeople(monthEvents, monthAbsences).length;
	const byDate = new Map<string, Set<string>>();
	for (const absence of monthAbsences) {
		const set = byDate.get(absence.date) ?? new Set();
		set.add(absence.email);
		byDate.set(absence.date, set);
	}
	return days.map((date) => {
		const absenceCount = byDate.get(date)?.size ?? 0;
		return { date, absenceCount, totalPeople: peopleCount };
	});
}
