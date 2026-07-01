import { createEvent, type Event as DayFlowEvent } from '@dayflow/core';
import {
	calendarDateTimeRangeChangesForStart,
	type CalendarDateTimeRangeFields,
	type CalendarDateTimeRangeStartInput
} from './calendar-date-time-range';
import { calendarParticipantsFromUnknown, type CalendarParticipant } from './calendar-participants';

export type CalendarMobileEditorDateTimeFields = CalendarDateTimeRangeFields;

export type CalendarMobileEditorStartDateTimeInput = CalendarDateTimeRangeStartInput;

export type CalendarMobileEditorUpdatedEventParams = {
	draftEvent: DayFlowEvent;
	title: string;
	description: string;
	start: Date;
	end: Date;
	allDay: boolean;
	calendarID: string;
	location: string;
	participants: CalendarParticipant[];
};

export function calendarMobileEditorStartDateTimeChanges(
	fields: CalendarMobileEditorDateTimeFields,
	input: CalendarMobileEditorStartDateTimeInput
): CalendarMobileEditorDateTimeFields {
	return calendarDateTimeRangeChangesForStart(fields, input);
}

export function calendarMobileEditorUpdatedEvent(params: CalendarMobileEditorUpdatedEventParams): DayFlowEvent {
	return createEvent({
		id: params.draftEvent.id,
		title: params.title.trim(),
		description: params.description.trim(),
		start: params.start,
		end: params.end,
		allDay: params.allDay,
		calendarId: params.calendarID,
		meta: {
			...(params.draftEvent.meta ?? {}),
			location: params.location.trim(),
			participants: calendarParticipantsFromUnknown(params.participants)
		}
	});
}
