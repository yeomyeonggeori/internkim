// 출결 팀 상세 팝업의 개인별 캘린더 일정 매칭을 담당한다.
import type { CalendarEvent } from '../../calendar/embed/calendar-event-persistence';
import { calendarParticipantsFromUnknown, type CalendarParticipant } from '../../calendar/embed/calendar-participants';
import { addDays, todayDateInTimeZone, utcDateKey } from '../shared/attendance-date';
import type { TeamStatusCalendarEventDetail, TeamStatusDayContextPerson } from './team-status-day-context';

type LegacyCalendarEvent = CalendarEvent & {
	people?: unknown;
};

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
	const legacyEvent = event as LegacyCalendarEvent;
	return calendarParticipantsFromUnknown([
		...(event.participants ?? []),
		...legacyParticipantsFromUnknown(legacyEvent.people)
	]);
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
	const personTokens = personMatchTokens(person);
	return participantTokens(participant).some((token) => personTokens.has(token));
}

function participantTokens(participant: CalendarParticipant): string[] {
	return [participant.personID, participant.name, participant.email ?? ''].map(normalizeToken).filter(Boolean);
}

function personMatchTokens(person: TeamStatusDayContextPerson): Set<string> {
	return new Set([person.email, person.displayName, person.mattermostUsername ?? ''].map(normalizeToken).filter(Boolean));
}

function normalizeToken(value: string): string {
	return value.trim().toLowerCase();
}

function isCalendarEventOnDate(event: CalendarEvent, date: string): boolean {
	const startDate = eventDateKey(event, event.startISO);
	const rawEndDate = eventDateKey(event, event.endISO);
	if (!startDate || !rawEndDate) return false;
	const endDate = event.isAllDay ? addDays(rawEndDate, -1) : rawEndDate;
	return date >= startDate && date <= endDate;
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
