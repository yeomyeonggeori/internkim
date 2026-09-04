import { defaultEventEndDate } from '$lib/calendar/default-event-duration';

export type CRMActivityCalendarDefaults = {
	calendarStart: string;
	calendarEnd: string;
};

export function localDateTimeValue(date: Date): string {
	const offset = date.getTimezoneOffset() * 60000;
	return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

export function crmActivityCalendarDefaults(occurredAt: string): CRMActivityCalendarDefaults {
	return {
		calendarStart: occurredAt,
		calendarEnd: localDateTimeValue(defaultEventEndDate(new Date(occurredAt)))
	};
}
