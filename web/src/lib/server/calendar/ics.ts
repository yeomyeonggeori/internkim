import type { CalendarEvent } from '../../../routes/calendar/embed/calendar-event-persistence';

const productIdentifier = '-//internkim//calendar//EN';
const longestLine = 75;

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

function eventLines(event: CalendarEvent, companyTimezone: string): string[] {
	const timezone = event.timeZone.trim() || companyTimezone;
	const lines = [
		'BEGIN:VEVENT',
		`UID:${event.uid}`,
		`DTSTAMP:${momentOf(event.updatedAt)}`,
		`SUMMARY:${escapedText(event.title)}`
	];
	if (event.isAllDay) {
		lines.push(`DTSTART;VALUE=DATE:${dayOf(event.startISO, timezone)}`);
		lines.push(`DTEND;VALUE=DATE:${dayAfter(event.endISO, timezone)}`);
	} else {
		lines.push(`DTSTART:${momentOf(event.startISO)}`);
		lines.push(`DTEND:${momentOf(event.endISO)}`);
	}
	const note = event.description.trim();
	if (note) lines.push(`DESCRIPTION:${escapedText(note)}`);
	const location = event.location.trim();
	if (location) lines.push(`LOCATION:${escapedText(location)}`);
	const color = event.color.trim();
	if (color) lines.push(`COLOR:${escapedText(color)}`);
	lines.push('END:VEVENT');
	return lines;
}

export function calendarFeedOf(events: CalendarEvent[], companyName: string, timezone: string): string {
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
