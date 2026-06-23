import type { AttendanceAbsence, AttendanceEvent } from '../attendance-context.svelte';
import { eachDayOfMonth } from './attendance-date';
import { uniquePeople } from './attendance-people';

export type DayHeatCell = {
	date: string;
	presentCount: number;
	totalPeople: number;
	level: 0 | 1 | 2 | 3;
};

export function computeHeatmap(month: string, events: AttendanceEvent[], absences: AttendanceAbsence[] = []): DayHeatCell[] {
	const days = eachDayOfMonth(month);
	const monthEvents = events.filter((event) => event.localDate.startsWith(month));
	const monthAbsences = absences.filter((absence) => absence.date.startsWith(month));
	const peopleCount = uniquePeople(monthEvents, monthAbsences).length;
	const byDate = new Map<string, Set<string>>();
	for (const event of monthEvents) {
		if (event.canceledAt) continue;
		if (event.kind !== 'clock_in') continue;
		const set = byDate.get(event.localDate) ?? new Set();
		set.add(event.email);
		byDate.set(event.localDate, set);
	}
	return days.map((date) => {
		const presentCount = byDate.get(date)?.size ?? 0;
		const ratio = peopleCount ? presentCount / peopleCount : 0;
		let level: DayHeatCell['level'] = 0;
		if (ratio > 0.75) level = 3;
		else if (ratio > 0.25) level = 2;
		else if (ratio > 0) level = 1;
		return { date, presentCount, totalPeople: peopleCount, level };
	});
}
