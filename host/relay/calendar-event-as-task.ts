// The importer that brought the device's calendar across and the relay that
// mirrors every later change write the same task, so they read an event here
// rather than each keeping its own copy of the answer.

export type DeviceCalendarParticipant = { personID?: string; name?: string; email?: string };

export type DeviceCalendarEvent = {
	id: string;
	uid: string;
	title: string;
	description?: string;
	location?: string;
	startISO: string;
	endISO: string;
	timeZone?: string;
	isAllDay?: boolean;
	color?: string;
	participants?: DeviceCalendarParticipant[];
	reminderLeadHours?: number;
	createdByEmail?: string;
	remoteSource?: string;
	remoteHref?: string;
};

export type DevicePerson = {
	userID?: string;
	name?: string;
	handle?: string;
	email?: string;
	image?: string;
};

export type EventMirror = { source: string; externalID: string; href?: string };

export type EventCalendar = { timeZone?: string; color?: string; mirrors: EventMirror[] };

export type EventAsTask = {
	title: string;
	startsAt: string;
	endsAt: string;
	isWholeDay: boolean;
	note: string | null;
	location: { name: string } | null;
	notifyMinutesBefore: number | null;
	calendar: EventCalendar;
};

export type ParticipantMatch = { email: string; by: 'personID' | 'email' | 'nameOrHandle' };

const deviceSource = 'internkim-device';

const untitledEvent = '(제목 없음)';

// A person's id reaches the calendar in two shapes: the account uuid, and the
// shorter one the calendar itself uses, which the organization API prints only
// inside the participant image path.
export function emailByPersonIDOf(people: DevicePerson[]): Map<string, string> {
	const directory = new Map<string, string>();
	for (const person of people) {
		const email = (person.email ?? '').toLowerCase();
		if (!email) continue;
		if (person.userID) directory.set(person.userID, email);
		const calendarID = /\/participants\/([0-9a-f]+)\/image/.exec(person.image ?? '')?.[1];
		if (calendarID) directory.set(calendarID, email);
	}
	return directory;
}

export function matchParticipant(
	participant: DeviceCalendarParticipant,
	people: DevicePerson[],
	emailByPersonID: Map<string, string>
): ParticipantMatch | undefined {
	const personID = (participant.personID ?? '').trim();
	if (personID) {
		const email = emailByPersonID.get(personID);
		if (email) return { email, by: 'personID' };
	}

	const email = participant.email?.trim().toLowerCase() ?? '';
	if (email) return { email, by: 'email' };

	const name = participant.name?.trim() ?? '';
	if (name.length < 2) return undefined;

	const named = people.filter((person) => person.name === name || person.handle === name);
	if (named.length !== 1 || !named[0].email) return undefined;
	return { email: named[0].email.toLowerCase(), by: 'nameOrHandle' };
}

export function titleOf(event: DeviceCalendarEvent): string {
	return event.title?.trim() || untitledEvent;
}

export function timeRunsBackwards(event: DeviceCalendarEvent): boolean {
	return !(Date.parse(event.endISO) >= Date.parse(event.startISO));
}

// The instants are written as the device recorded them. An all-day event there is
// a midnight-to-midnight range in its own zone, so the raw instants plus the zone
// reproduce it, where deriving a date would move it.
export function eventAsTask(event: DeviceCalendarEvent): EventAsTask {
	return {
		title: titleOf(event),
		startsAt: event.startISO,
		endsAt: event.endISO,
		isWholeDay: Boolean(event.isAllDay),
		note: event.description?.trim() || null,
		location: event.location?.trim() ? { name: event.location.trim() } : null,
		notifyMinutesBefore: reminderMinutesOf(event.reminderLeadHours),
		calendar: calendarOf(event)
	};
}

function reminderMinutesOf(leadHours: number | undefined): number | null {
	if (typeof leadHours !== 'number' || !Number.isFinite(leadHours) || leadHours <= 0) return null;
	return Math.round(leadHours * 60);
}

function calendarOf(event: DeviceCalendarEvent): EventCalendar {
	const externalID = event.uid || event.id;
	const mirrors: EventMirror[] = [{ source: deviceSource, externalID }];
	if (event.remoteSource && event.remoteHref) {
		mirrors.push({ source: event.remoteSource, externalID, href: event.remoteHref });
	}
	return {
		...(event.timeZone ? { timeZone: event.timeZone } : {}),
		...(event.color ? { color: event.color } : {}),
		mirrors
	};
}
