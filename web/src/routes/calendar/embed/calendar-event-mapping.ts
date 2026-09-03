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
	color: string,
	timeZone: string
): CalendarEventPayload {
	const startDate = calendarDateFromDayTaskEventStart(event);
	const endDate = calendarDateFromDayTaskEventEnd(event);
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

function calendarDateFromDayTaskEventStart(event: DayTaskEvent): Date {
	if (event.allDay) return dateFromCalendarDateParts(calendarDatePartsFromDate(event.start));
	return new Date(event.start);
}

function calendarDateFromDayTaskEventEnd(event: DayTaskEvent): Date {
	if (event.allDay) return dateFromCalendarDateParts(nextCalendarDateParts(calendarDatePartsFromDate(event.end)));
	return new Date(event.end);
}

function calendarDatePartsFromDate(value: Date): CalendarDateParts {
	return {
		year: value.getFullYear(),
		month: value.getMonth() + 1,
		day: value.getDate()
	};
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

function calendarDatePartsFromISODate(isoDate: string): CalendarDateParts {
	const matches = /^(\d{4})-(\d{2})-(\d{2})$/.exec(isoDate);
	if (!matches) throw new Error(`Invalid calendar date: ${isoDate}`);
	return {
		year: Number(matches[1]),
		month: Number(matches[2]),
		day: Number(matches[3])
	};
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
