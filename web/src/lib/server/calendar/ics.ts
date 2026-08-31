export type EventCalendarFields = {
	timeZone?: string;
	color?: string;
};

export type CalendarFeedEvent = {
	id: string;
	title: string;
	note: string | null;
	location: unknown;
	starts_at: string;
	ends_at: string;
	is_whole_day: boolean;
	updated_at: string;
	calendar: unknown;
};

export function calendarFieldsOf(carried: unknown): EventCalendarFields {
	if (typeof carried !== 'object' || carried === null) return {};
	const { timeZone, color } = carried as Record<string, unknown>;
	return {
		timeZone: typeof timeZone === 'string' && timeZone.trim() ? timeZone.trim() : undefined,
		color: typeof color === 'string' && color.trim() ? color.trim() : undefined
	};
}

const productIdentifier = '-//internkim//calendar//EN';
const longestLine = 75;

export function locationNameOf(location: unknown): string {
	if (typeof location === 'string') return location;
	if (typeof location === 'object' && location !== null) {
		const named = (location as { name?: unknown }).name;
		if (typeof named === 'string') return named;
	}
	return '';
}

export function escapedText(written: string): string {
	return written
		.replace(/\\/g, '\\\\')
		.replace(/;/g, '\\;')
		.replace(/,/g, '\\,')
		.replace(/\r?\n/g, '\\n');
}

// RFC 5545 counts octets, not characters, so a line is measured as it is sent.
export function foldedLine(line: string): string {
	const bytes = new TextEncoder().encode(line);
	if (bytes.byteLength <= longestLine) return line;

	const decoder = new TextDecoder();
	const folded: string[] = [];
	let taken = 0;
	while (taken < bytes.byteLength) {
		const room = folded.length === 0 ? longestLine : longestLine - 1;
		let size = Math.min(room, bytes.byteLength - taken);
		while (size > 1 && (bytes[taken + size] & 0b1100_0000) === 0b1000_0000) size -= 1;
		folded.push(decoder.decode(bytes.subarray(taken, taken + size)));
		taken += size;
	}
	return folded.join('\r\n ');
}

export function momentOf(instant: string): string {
	return new Date(instant).toISOString().replace(/[-:]/g, '').replace(/\.\d{3}/, '');
}

export function dayOf(instant: string, timezone: string): string {
	return new Intl.DateTimeFormat('en-CA', {
		timeZone: timezone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	})
		.format(new Date(instant))
		.replace(/-/g, '');
}

// A whole-day event ends the day after the last one it covers, because DTEND is
// the moment it stops rather than the last day it holds.
function dayAfter(instant: string, timezone: string): string {
	return dayOf(new Date(new Date(instant).getTime() + 24 * 60 * 60 * 1000).toISOString(), timezone);
}

function eventLines(event: CalendarFeedEvent, companyTimezone: string): string[] {
	const carried = calendarFieldsOf(event.calendar);
	const timezone = carried.timeZone ?? companyTimezone;
	const lines = [
		'BEGIN:VEVENT',
		`UID:${event.id}`,
		`DTSTAMP:${momentOf(event.updated_at)}`,
		`SUMMARY:${escapedText(event.title)}`
	];
	if (event.is_whole_day) {
		lines.push(`DTSTART;VALUE=DATE:${dayOf(event.starts_at, timezone)}`);
		lines.push(`DTEND;VALUE=DATE:${dayAfter(event.ends_at, timezone)}`);
	} else {
		lines.push(`DTSTART:${momentOf(event.starts_at)}`);
		lines.push(`DTEND:${momentOf(event.ends_at)}`);
	}
	const note = (event.note ?? '').trim();
	if (note) lines.push(`DESCRIPTION:${escapedText(note)}`);
	const location = locationNameOf(event.location).trim();
	if (location) lines.push(`LOCATION:${escapedText(location)}`);
	if (carried.color) lines.push(`COLOR:${escapedText(carried.color)}`);
	lines.push('END:VEVENT');
	return lines;
}

export function calendarFeedOf(events: CalendarFeedEvent[], companyName: string, timezone: string): string {
	const lines = [
		'BEGIN:VCALENDAR',
		'VERSION:2.0',
		`PRODID:${productIdentifier}`,
		'CALSCALE:GREGORIAN',
		'METHOD:PUBLISH',
		`X-WR-CALNAME:${escapedText(companyName)}`,
		`X-WR-TIMEZONE:${timezone}`
	];
	for (const event of events) lines.push(...eventLines(event, timezone));
	lines.push('END:VCALENDAR');
	return lines.map(foldedLine).join('\r\n') + '\r\n';
}
