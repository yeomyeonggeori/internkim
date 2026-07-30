export type CalendarParticipantInput = {
	personID: string;
	name: string;
	email?: string;
};

export type CalendarParticipant = CalendarParticipantInput & {
	image?: string;
};

export async function fetchCalendarParticipants(errorFallback: string): Promise<CalendarParticipant[]> {
	const response = await fetch('/calendar/api/participants', { credentials: 'include' });
	if (!response.ok) throw new Error(await responseErrorMessage(response, errorFallback));
	const document = await response.json();
	return calendarParticipantsFromUnknown(objectProperty(document, 'participants'));
}

export function calendarParticipantsFromUnknown(value: unknown): CalendarParticipant[] {
	if (!Array.isArray(value)) return [];
	return normalizeCalendarParticipants(value.map(calendarParticipantFromUnknown).filter(isCalendarParticipant));
}

export function calendarParticipantNames(participants: CalendarParticipant[]): string[] {
	return participants.map((participant) => participant.name);
}

export function calendarParticipantsWithViewerFirst(
	participants: CalendarParticipant[],
	viewerEmail: string
): CalendarParticipant[] {
	const normalizedViewerEmail = viewerEmail.trim().toLowerCase();
	if (!normalizedViewerEmail) return participants;
	const viewerParticipants = participants.filter(
		(participant) => (participant.email ?? '').trim().toLowerCase() === normalizedViewerEmail
	);
	if (viewerParticipants.length === 0) return participants;
	return [
		...viewerParticipants,
		...participants.filter((participant) => !viewerParticipants.includes(participant))
	];
}

export function calendarParticipantInputs(participants: CalendarParticipant[]): CalendarParticipantInput[] {
	return normalizeCalendarParticipants(participants).map((participant) => ({
		personID: participant.personID,
		name: participant.name,
		email: participant.email
	}));
}

export function calendarParticipantKey(participant: CalendarParticipant): string {
	if (participant.personID !== '') return `id:${participant.personID.toLowerCase()}`;
	if ((participant.email ?? '') !== '') return `email:${participant.email?.toLowerCase()}`;
	if (participant.name !== '') return `name:${participant.name.toLowerCase()}`;
	return '';
}

export function calendarParticipantOptionLabel(participant: CalendarParticipant, participants: CalendarParticipant[]): string {
	if (!hasCalendarParticipantDuplicateName(participant, participants)) return participant.name;
	if ((participant.email ?? '') !== '') return `${participant.name} · ${participant.email}`;
	if (participant.personID !== '') return `${participant.name} · ${participant.personID}`;
	return participant.name;
}

export function calendarParticipantMatchesSearch(participant: CalendarParticipant, searchText: string): boolean {
	const normalizedSearchText = searchText.trim().toLowerCase();
	if (normalizedSearchText === '') return true;
	return [participant.name, participant.email ?? '', participant.personID].some((value) =>
		value.toLowerCase().includes(normalizedSearchText)
	);
}

export function calendarParticipantsEqual(left: CalendarParticipant[], right: CalendarParticipant[]): boolean {
	const leftParticipants = normalizeCalendarParticipants(left);
	const rightParticipants = normalizeCalendarParticipants(right);
	if (leftParticipants.length !== rightParticipants.length) return false;
	return leftParticipants.every((participant, index) => {
		const otherParticipant = rightParticipants[index];
		return (
			participant.personID === otherParticipant.personID &&
			participant.name === otherParticipant.name &&
			(participant.email ?? '') === (otherParticipant.email ?? '')
		);
	});
}

function calendarParticipantFromUnknown(value: unknown): CalendarParticipant | null {
	const personID = stringProperty(value, 'personID');
	const name = stringProperty(value, 'name');
	const email = stringProperty(value, 'email');
	const image = stringProperty(value, 'image');
	if (name === '') return null;
	return { personID, name, email, image };
}

function normalizeCalendarParticipants(values: CalendarParticipant[]): CalendarParticipant[] {
	const participants: CalendarParticipant[] = [];
	const seenKeys = new Set<string>();
	for (const value of values) {
		const image = value.image?.trim();
		const participant = {
			personID: value.personID.trim(),
			name: value.name.trim(),
			email: value.email?.trim().toLowerCase(),
			image: image === '' ? undefined : image
		};
		const key = calendarParticipantKey(participant);
		if (key === '' || seenKeys.has(key)) continue;
		seenKeys.add(key);
		participants.push(participant);
	}
	return participants;
}

function hasCalendarParticipantDuplicateName(participant: CalendarParticipant, participants: CalendarParticipant[]): boolean {
	const normalizedName = participant.name.trim().toLowerCase();
	if (normalizedName === '') return false;
	return participants.some((candidate) => {
		if (calendarParticipantKey(candidate) === calendarParticipantKey(participant)) return false;
		return candidate.name.trim().toLowerCase() === normalizedName;
	});
}

function isCalendarParticipant(value: CalendarParticipant | null | undefined): value is CalendarParticipant {
	return value !== null && value !== undefined;
}

function stringProperty(value: unknown, propertyName: string): string {
	const propertyValue = objectProperty(value, propertyName);
	return typeof propertyValue === 'string' ? propertyValue.trim() : '';
}

function objectProperty(value: unknown, propertyName: string): unknown {
	if (!value || typeof value !== 'object') return undefined;
	return Reflect.get(value, propertyName);
}

async function responseErrorMessage(response: Response, fallback: string): Promise<string> {
	const message = (await response.text()).trim();
	if (!message || message.startsWith('<!doctype html>') || message.startsWith('<html')) return fallback;
	return message;
}
