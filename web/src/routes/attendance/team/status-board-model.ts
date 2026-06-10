import type { AttendanceEvent, AttendancePresence } from '../attendance-context.svelte';
import type { PersonStatus } from '../shared/attendance-aggregation';
import { uniquePeople } from '../shared/attendance-aggregation';

export type PresenceFilter = AttendancePresence | 'all';

export type PresencePerson = {
	email: string;
	displayName: string;
	presence: AttendancePresence;
};

export type TeamStatusCounts = Record<PersonStatus, number>;

export type TeamStatusPerson = {
	status: PersonStatus;
};

export function resolveDefaultDate(
	month: string | undefined,
	events: Pick<AttendanceEvent, 'localDate' | 'kind' | 'canceledAt'>[],
	today: string
): string {
	if (!month) return today;
	if (today.startsWith(month)) return today;
	const dates = events
		.filter((event) => event.kind === 'clock_in' && !event.canceledAt && event.localDate.startsWith(month))
		.map((event) => event.localDate)
		.sort();
	if (dates.length) return dates[0];
	return `${month}-01`;
}

export function summarizePeople(people: TeamStatusPerson[]): TeamStatusCounts {
	const counts: TeamStatusCounts = {
		working: 0,
		finished: 0,
		absence: 0,
		absent: 0,
		weekend: 0,
		upcoming: 0
	};
	for (const person of people) {
		counts[person.status] += 1;
	}
	return counts;
}

export function buildPresencePeople(
	events: Parameters<typeof uniquePeople>[0],
	presences: Record<string, AttendancePresence>
): PresencePerson[] {
	return uniquePeople(events)
		.map((person) => {
			const presence = presences[person.email];
			return presence ? { email: person.email, displayName: person.displayName, presence } : null;
		})
		.filter((person): person is PresencePerson => person !== null);
}

export function filterPresencePeople(
	people: PresencePerson[],
	presenceFilter: PresenceFilter
): PresencePerson[] {
	return presenceFilter === 'all'
		? people
		: people.filter((person) => person.presence === presenceFilter);
}

export function summarizePresences(people: PresencePerson[]): Record<AttendancePresence, number> {
	const counts: Record<AttendancePresence, number> = {
		online: 0,
		away: 0,
		dnd: 0,
		offline: 0
	};
	for (const person of people) {
		counts[person.presence] += 1;
	}
	return counts;
}

export function presenceDotClass(presence: AttendancePresence): string {
	switch (presence) {
		case 'online':
			return 'bg-emerald-500';
		case 'away':
			return 'bg-amber-500';
		case 'dnd':
			return 'bg-red-500';
		case 'offline':
			return 'bg-muted-foreground/40';
	}
}
