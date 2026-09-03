import { createCalendarModelEvent as createEvent, type CalendarModelEvent as DayTaskEvent } from './calendar-event-model';
import type { CalendarEvent, CalendarEventPayload } from './calendar-event-persistence';
import type { CalendarHoliday } from './calendar-holiday-persistence';
import { calendarParticipantInputs, calendarParticipantsFromUnknown } from './calendar-participants';
import { eventReminderLeadOf } from '$lib/calendar/event-reminder-lead';

export type CalendarDateParts = {
	year: number;
	month: number;
	day: number;
};

export function dayTaskEventFromCalendarEvent(event: CalendarEvent): DayTaskEvent {
	const timeZone = event.timeZone.trim() || undefined;
	return createEvent({
		id: event.id,
		title: event.title,
		description: event.description,
		start: event.isAllDay ? localDateFromISODate(event.startISO, timeZone) : new Date(event.startISO),
		end: event.isAllDay ? dayBeforeLocalISODate(event.endISO, timeZone) : new Date(event.endISO),
		allDay: event.isAllDay,
		calendarId: 'internkim',
		meta: {
			uid: event.uid,
			location: event.location,
			color: event.color,
			participants: calendarParticipantsFromUnknown(event.participants),
			reminderMinutesBefore: eventReminderLeadOf(event.reminderMinutesBefore),
			timeZone: event.timeZone,
			createdByEmail: event.createdByEmail,
			createdByName: event.createdByName,
			createdByImage: event.createdByImage ?? '',
			updatedByEmail: event.updatedByEmail ?? '',
			updatedByName: event.updatedByName ?? '',
			updatedByImage: event.updatedByImage ?? '',
			updatedByAt: event.updatedByAt ?? '',
			updatedAt: event.updatedAt,
			readOnly: event.readOnly ?? false,
			source: event.source ?? ''
		}
	});
}

export function dayTaskEventFromCalendarHoliday(holiday: CalendarHoliday): DayTaskEvent {
	const date = localDateFromCalendarDateParts(calendarDatePartsFromISODate(holiday.date));
	return createEvent({
		id: holiday.id,
		title: holiday.title,
		description: '',
		start: date,
		end: date,
		allDay: true,
		calendarId: 'holidays',
		meta: {
			color: holiday.color,
			countryCode: holiday.countryCode ?? '',
			readOnly: true,
			source: holiday.source
		}
	});
}

export function calendarEventPayloadFromDayTaskEvent(
	event: DayTaskEvent,
	color: string
): CalendarEventPayload {
	return {
		eventID: event.id,
		title: event.title || 'Untitled event',
		description: event.description ?? '',
		location: typeof event.meta?.location === 'string' ? event.meta.location : '',
		startsAt: calendarEdgeWritten(event.start, event.allDay ?? false),
		endsAt: calendarEdgeWritten(event.end, event.allDay ?? false),
		isAllDay: event.allDay ?? false,
		color,
		reminderMinutesBefore: eventReminderLeadOf(event.meta?.reminderMinutesBefore),
		participants: calendarParticipantInputs(calendarParticipantsFromUnknown(event.meta?.participants))
	};
}

export function eventStartDate(event: DayTaskEvent): Date {
	return event.allDay ? localDateFromCalendarDateParts(calendarDatePartsFromDate(event.start)) : new Date(event.start);
}

export function eventEndDate(event: DayTaskEvent): Date {
	return event.allDay ? localDateFromCalendarDateParts(calendarDatePartsFromDate(event.end)) : new Date(event.end);
}

function calendarEdgeWritten(edge: Date, isWholeDay: boolean): string {
	if (!isWholeDay) return edge.toISOString();
	return dayWritten(calendarDatePartsFromDate(edge));
}

function dayWritten(dateParts: CalendarDateParts): string {
	const month = String(dateParts.month).padStart(2, '0');
	const day = String(dateParts.day).padStart(2, '0');
	return `${dateParts.year}-${month}-${day}`;
}

function calendarDatePartsFromDate(value: Date): CalendarDateParts {
	return {
		year: value.getFullYear(),
		month: value.getMonth() + 1,
		day: value.getDate()
	};
}

function localDateFromCalendarDateParts(dateParts: CalendarDateParts): Date {
	return new Date(dateParts.year, dateParts.month - 1, dateParts.day);
}

function calendarDatePartsInTimeZone(instant: string, timeZone: string | undefined): CalendarDateParts {
	const parts = new Intl.DateTimeFormat('en-CA', {
		timeZone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).formatToParts(new Date(instant));
	const valueOf = (partType: string) => Number(parts.find((part) => part.type === partType)?.value);
	return { year: valueOf('year'), month: valueOf('month'), day: valueOf('day') };
}

function localDateFromISODate(isoDate: string, timeZone: string | undefined): Date {
	return localDateFromCalendarDateParts(calendarDatePartsInTimeZone(isoDate, timeZone));
}

function calendarDatePartsFromISODate(isoDate: string): CalendarDateParts {
	const matches = /^(\d{4})-(\d{2})-(\d{2})$/.exec(isoDate);
	if (!matches) throw new Error(`Invalid calendar date: ${isoDate}`);
	return {
		year: Number(matches[1]),
		month: Number(matches[2]),
		day: Number(matches[3])
	};
}

function dayBeforeLocalISODate(isoDate: string, timeZone: string | undefined): Date {
	const date = localDateFromISODate(isoDate, timeZone);
	date.setDate(date.getDate() - 1);
	return date;
}
