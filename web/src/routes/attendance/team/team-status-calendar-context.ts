import type { CalendarEvent } from '../../calendar/embed/calendar-event-persistence';
import { calendarParticipantsFromUnknown, type CalendarParticipant } from '../../calendar/embed/calendar-participants';
import { addDays, timeInTimeZone, todayDateInTimeZone, utcDateKey } from '../shared/attendance-date';
import type { TeamStatusCalendarEventDetail, TeamStatusDayContextPerson } from './team-status-day-context';

export function calendarEventDetailsForPersonDay(
	person: TeamStatusDayContextPerson,
	date: string,
	calendarEvents: CalendarEvent[],
	localeCode: string,
	allDayLabel: string
): TeamStatusCalendarEventDetail[] {
	return calendarEvents
		.filter((event) => isCalendarEventVisibleForPersonDay(event, person, date))
		.map((event) => calendarEventDetail(event, localeCode, allDayLabel));
}

function isCalendarEventVisibleForPersonDay(
	event: CalendarEvent,
	person: TeamStatusDayContextPerson,
	date: string
): boolean {
	const participants = eventParticipants(event);
	if (participants.length === 0) return false;
	if (participants.some(isAllParticipant)) return false;
	if (!isCalendarEventOnDate(event, date)) return false;
	return participants.some((participant) => participantMatchesPerson(participant, person));
}

function eventParticipants(event: CalendarEvent): CalendarParticipant[] {
	return calendarParticipantsFromUnknown([
		...(event.participants ?? []),
		...legacyParticipantsFromUnknown(legacyPeopleFromEvent(event))
	]);
}

function legacyPeopleFromEvent(event: CalendarEvent): unknown {
	if (!('people' in event)) return undefined;
	return event.people;
}

function legacyParticipantsFromUnknown(value: unknown): CalendarParticipant[] {
	if (!Array.isArray(value)) return [];
	return value.flatMap((item) => {
		if (typeof item === 'string') return [{ personID: '', name: item }];
		return calendarParticipantsFromUnknown([item]);
	});
}

function isAllParticipant(participant: CalendarParticipant): boolean {
	return participantTokens(participant).some((token) => token === 'all');
}

function participantMatchesPerson(participant: CalendarParticipant, person: TeamStatusDayContextPerson): boolean {
	const exactPersonTokens = exactPersonMatchTokens(person);
	const exactParticipantTokens = exactParticipantMatchTokens(participant);
	if (exactPersonTokens.size > 0 && exactParticipantTokens.length > 0) {
		return exactParticipantTokens.some((token) => exactPersonTokens.has(token));
	}
	const nameToken = normalizeToken(participant.name);
	return Boolean(nameToken) && fallbackPersonNameTokens(person).has(nameToken);
}

function participantTokens(participant: CalendarParticipant): string[] {
	return [participant.personID, participant.name, participant.email ?? ''].map(normalizeToken).filter(Boolean);
}

function exactParticipantMatchTokens(participant: CalendarParticipant): string[] {
	return [participant.personID, participant.email ?? ''].map(normalizeToken).filter(Boolean);
}

function exactPersonMatchTokens(person: TeamStatusDayContextPerson): Set<string> {
	return new Set([person.email, person.mattermostUsername ?? ''].map(normalizeToken).filter(Boolean));
}

function fallbackPersonNameTokens(person: TeamStatusDayContextPerson): Set<string> {
	return new Set([person.displayName, person.mattermostUsername ?? ''].map(normalizeToken).filter(Boolean));
}

function normalizeToken(value: string): string {
	return value.trim().toLowerCase();
}

function isCalendarEventOnDate(event: CalendarEvent, date: string): boolean {
	const startDate = eventDateKey(event, event.startISO);
	const rawEndDate = eventDateKey(event, event.endISO);
	if (!startDate || !rawEndDate) return false;
	const endDate = isExclusiveEndDate(event) ? addDays(rawEndDate, -1) : rawEndDate;
	return date >= startDate && date <= endDate;
}

function isExclusiveEndDate(event: CalendarEvent): boolean {
	if (event.isAllDay) return true;
	const endDate = new Date(event.endISO);
	if (Number.isNaN(endDate.getTime())) return false;
	const endTime = timeInTimeZone(event.timeZone, endDate);
	return endTime === '00:00' || endTime === '24:00';
}

function eventDateKey(event: CalendarEvent, isoDate: string): string {
	const date = new Date(isoDate);
	if (Number.isNaN(date.getTime())) return '';
	if (event.isAllDay) return utcDateKey(date);
	return todayDateInTimeZone(event.timeZone, date);
}

function calendarEventDetail(event: CalendarEvent, localeCode: string, allDayLabel: string): TeamStatusCalendarEventDetail {
	return {
		id: event.id,
		title: event.title || '-',
		timeLabel: calendarEventTimeLabel(event, localeCode, allDayLabel),
		location: event.location || undefined
	};
}

function calendarEventTimeLabel(event: CalendarEvent, localeCode: string, allDayLabel: string): string {
	if (event.isAllDay) return allDayLabel;
	const formatter = new Intl.DateTimeFormat(localeCode, {
		timeZone: event.timeZone || undefined,
		hour: '2-digit',
		minute: '2-digit',
		hour12: false
	});
	return `${formatter.format(new Date(event.startISO))}-${formatter.format(new Date(event.endISO))}`;
}
