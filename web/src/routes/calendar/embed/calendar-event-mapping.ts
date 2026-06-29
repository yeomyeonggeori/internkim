import { createEvent, temporalToDate, type Event as DayFlowEvent } from '@dayflow/core';
import type { CalendarEvent, CalendarEventPayload } from './calendar-event-persistence';
import { calendarParticipantInputs, calendarParticipantsFromUnknown } from './calendar-participants';

export type CalendarDateParts = {
	year: number;
	month: number;
	day: number;
};

export function dayFlowEventFromCalendarEvent(event: CalendarEvent): DayFlowEvent {
	return createEvent({
		id: event.id,
		title: event.title,
		description: event.description,
		start: event.isAllDay ? localDateFromISODate(event.startISO) : new Date(event.startISO),
		end: event.isAllDay ? dayBeforeLocalISODate(event.endISO) : new Date(event.endISO),
		allDay: event.isAllDay,
		calendarId: 'internkim',
		meta: {
			uid: event.uid,
			location: event.location,
			color: event.color,
			participants: calendarParticipantsFromUnknown(event.participants),
			timeZone: event.timeZone,
			createdByEmail: event.createdByEmail,
			createdByName: event.createdByName,
			createdByImage: event.createdByImage ?? '',
			updatedByEmail: event.updatedByEmail ?? '',
			updatedByName: event.updatedByName ?? '',
			updatedByImage: event.updatedByImage ?? '',
			updatedByAt: event.updatedByAt ?? '',
			updatedAt: event.updatedAt
		}
	});
}

export function calendarEventPayloadFromDayFlowEvent(
	event: DayFlowEvent,
	color: string,
	timeZone: string
): CalendarEventPayload {
	const startDate = calendarDateFromDayFlowEventStart(event);
	const endDate = calendarDateFromDayFlowEventEnd(event);
	return {
		eventID: event.id,
		title: event.title || 'Untitled event',
		description: event.description ?? '',
		location: typeof event.meta?.location === 'string' ? event.meta.location : '',
		startISO: startDate.toISOString(),
		endISO: endDate.toISOString(),
		timeZone,
		isAllDay: event.allDay ?? false,
		color,
		participants: calendarParticipantInputs(calendarParticipantsFromUnknown(event.meta?.participants))
	};
}

export function eventStartDate(event: DayFlowEvent): Date {
	return event.allDay ? localDateFromCalendarDateParts(calendarDatePartsFromTemporal(event.start)) : temporalToDate(event.start);
}

export function eventEndDate(event: DayFlowEvent): Date {
	return event.allDay ? localDateFromCalendarDateParts(calendarDatePartsFromTemporal(event.end)) : temporalToDate(event.end);
}

function calendarDateFromDayFlowEventStart(event: DayFlowEvent): Date {
	if (event.allDay) return dateFromCalendarDateParts(calendarDatePartsFromTemporal(event.start));
	return temporalToDate(event.start);
}

function calendarDateFromDayFlowEventEnd(event: DayFlowEvent): Date {
	if (event.allDay) return dateFromCalendarDateParts(nextCalendarDateParts(calendarDatePartsFromTemporal(event.end)));
	return temporalToDate(event.end);
}

function calendarDatePartsFromTemporal(value: DayFlowEvent['start']): CalendarDateParts {
	if (isCalendarDateParts(value)) return value;
	const date = temporalToDate(value);
	return {
		year: date.getFullYear(),
		month: date.getMonth() + 1,
		day: date.getDate()
	};
}

function isCalendarDateParts(value: unknown): value is CalendarDateParts {
	if (!value || typeof value !== 'object') return false;
	return 'year' in value && 'month' in value && 'day' in value;
}

function dateFromCalendarDateParts(dateParts: CalendarDateParts): Date {
	return new Date(Date.UTC(dateParts.year, dateParts.month - 1, dateParts.day));
}

function localDateFromCalendarDateParts(dateParts: CalendarDateParts): Date {
	return new Date(dateParts.year, dateParts.month - 1, dateParts.day);
}

function localDateFromISODate(isoDate: string): Date {
	const date = new Date(isoDate);
	return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function dayBeforeLocalISODate(isoDate: string): Date {
	const date = localDateFromISODate(isoDate);
	date.setDate(date.getDate() - 1);
	return date;
}

function nextCalendarDateParts(dateParts: CalendarDateParts): CalendarDateParts {
	const date = new Date(dateParts.year, dateParts.month - 1, dateParts.day);
	date.setDate(date.getDate() + 1);
	return {
		year: date.getFullYear(),
		month: date.getMonth() + 1,
		day: date.getDate()
	};
}
