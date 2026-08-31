import { dayIn } from '../public-api/record/days';

export type CalendarFeedEvent = {
	id: string;
	title: string;
	note: string | null;
	location: string | null;
	startsAt: string;
	endsAt: string;
	isWholeDay: boolean;
	updatedAt: string;
};

export type CalendarFeed = {
	company: string;
	timezone: string;
	events: CalendarFeedEvent[];
};

const productIdentifier = '-//internkim//calendar feed//EN';
const octetsPerFoldedLine = 75;

export function icsDocumentOf(feed: CalendarFeed, uidDomain: string): string {
	const lines = [
		'BEGIN:VCALENDAR',
		'VERSION:2.0',
		`PRODID:${productIdentifier}`,
		'CALSCALE:GREGORIAN',
		'METHOD:PUBLISH',
		`X-WR-CALNAME:${escapedText(feed.company)}`,
		`X-WR-TIMEZONE:${feed.timezone}`,
		...feed.events.flatMap((event) => eventLines(event, feed.timezone, uidDomain)),
		'END:VCALENDAR'
	];
	return `${lines.flatMap(foldedLine).join('\r\n')}\r\n`;
}

function eventLines(event: CalendarFeedEvent, timezone: string, uidDomain: string): string[] {
	return [
		'BEGIN:VEVENT',
		`UID:${event.id}@${uidDomain}`,
		`DTSTAMP:${utcStampOf(event.updatedAt)}`,
		`LAST-MODIFIED:${utcStampOf(event.updatedAt)}`,
		...boundaryLines(event, timezone),
		`SUMMARY:${escapedText(event.title)}`,
		...(event.note ? [`DESCRIPTION:${escapedText(event.note)}`] : []),
		...(event.location ? [`LOCATION:${escapedText(event.location)}`] : []),
		'END:VEVENT'
	];
}

function boundaryLines(event: CalendarFeedEvent, timezone: string): string[] {
	if (!event.isWholeDay) {
		return [`DTSTART:${utcStampOf(event.startsAt)}`, `DTEND:${utcStampOf(event.endsAt)}`];
	}
	return [
		`DTSTART;VALUE=DATE:${dateStampOf(event.startsAt, timezone)}`,
		`DTEND;VALUE=DATE:${dateStampOf(event.endsAt, timezone)}`
	];
}

function utcStampOf(moment: string): string {
	return new Date(moment).toISOString().replace(/\.\d+/, '').replaceAll('-', '').replaceAll(':', '');
}

function dateStampOf(moment: string, timezone: string): string {
	return dayIn(timezone, new Date(moment)).replaceAll('-', '');
}

function escapedText(value: string): string {
	return value
		.replaceAll('\\', '\\\\')
		.replaceAll(';', '\\;')
		.replaceAll(',', '\\,')
		.replaceAll('\r\n', '\\n')
		.replaceAll('\n', '\\n')
		.replaceAll('\r', '\\n');
}

function foldedLine(line: string): string[] {
	const octets = new TextEncoder().encode(line);
	if (octets.length <= octetsPerFoldedLine) return [line];

	const decoder = new TextDecoder();
	const pieces: string[] = [];
	let taken = 0;
	while (taken < octets.length) {
		const continuation = pieces.length > 0;
		const room = continuation ? octetsPerFoldedLine - 1 : octetsPerFoldedLine;
		const size = wholeCharacterOctets(octets, taken, room);
		const piece = decoder.decode(octets.subarray(taken, taken + size));
		pieces.push(continuation ? ` ${piece}` : piece);
		taken += size;
	}
	return pieces;
}

function wholeCharacterOctets(octets: Uint8Array, from: number, room: number): number {
	const remaining = octets.length - from;
	if (remaining <= room) return remaining;
	let size = room;
	while (size > 0 && isContinuationOctet(octets[from + size])) size -= 1;
	return size > 0 ? size : room;
}

function isContinuationOctet(octet: number): boolean {
	return (octet & 0b1100_0000) === 0b1000_0000;
}
